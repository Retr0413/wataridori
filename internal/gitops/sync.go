package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// SyncOptions locates the manifest repository a server follows.
type SyncOptions struct {
	// URL is the remote to clone or fetch from.
	URL string
	// Branch is the branch to track. The manifest lives in the repository, so
	// a repository-level branch is needed to read it at all; per-environment
	// branch following would need one checkout per branch (see issue #38).
	Branch string
	// Dir is the working copy. It is managed by Wataridori: Sync fast-forwards
	// it, so it is not a place to keep local edits.
	Dir string
	// Token authenticates against HTTPS remotes (GitHub PAT or installation
	// token). Empty means anonymous, which works for public repositories.
	Token string
}

func (o SyncOptions) auth() transport.AuthMethod {
	if o.Token == "" {
		return nil
	}
	// GitHub accepts any non-empty username with a token as the password.
	return &githttp.BasicAuth{Username: "x-access-token", Password: o.Token}
}

// ErrLocalAhead reports that the working copy holds commits the remote does
// not, so fast-forwarding it would throw them away.
var ErrLocalAhead = errors.New("working copy has commits not on the remote")

// ErrLocalDirty reports uncommitted changes to tracked files.
var ErrLocalDirty = errors.New("working copy has uncommitted changes")

// Sync makes Dir a checkout of Branch at the remote's tip and returns the
// resulting commit id. It clones when Dir holds no repository, and otherwise
// fetches and fast-forwards.
//
// Sync never discards local work. A promotion commits into this same working
// copy, and a plain "reset --hard" would silently delete a promotion that had
// not been pushed yet. So a working copy that is dirty or ahead of the remote
// is reported as an error instead: the caller (the reconcile loop) skips the
// cycle and logs it, which is loud and recoverable, where losing a promotion
// is neither.
func Sync(ctx context.Context, opts SyncOptions) (string, error) {
	if opts.URL == "" || opts.Branch == "" || opts.Dir == "" {
		return "", fmt.Errorf("sync: URL, Branch and Dir are all required")
	}

	repo, err := openOrClone(ctx, opts)
	if err != nil {
		return "", err
	}

	remoteRef, err := fetchBranch(ctx, repo, opts)
	if err != nil {
		return "", err
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("reading HEAD of %s: %w", opts.Dir, err)
	}
	if head.Hash() == remoteRef.Hash() {
		return head.Hash().String(), nil // already at the remote tip
	}

	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	if err := checkClean(wt); err != nil {
		return "", err
	}
	if err := checkFastForward(repo, head.Hash(), remoteRef.Hash()); err != nil {
		return "", err
	}

	if err := wt.Reset(&git.ResetOptions{Mode: git.HardReset, Commit: remoteRef.Hash()}); err != nil {
		return "", fmt.Errorf("fast-forwarding %s to %s: %w", opts.Dir, opts.Branch, err)
	}
	return remoteRef.Hash().String(), nil
}

// openOrClone returns the repository in Dir, cloning it when absent. An empty
// or missing directory is normal on a fresh server instance, whose container
// starts with no checkout at all.
func openOrClone(ctx context.Context, opts SyncOptions) (*git.Repository, error) {
	repo, err := git.PlainOpen(opts.Dir)
	if err == nil {
		return repo, nil
	}
	if !errors.Is(err, git.ErrRepositoryNotExists) {
		return nil, fmt.Errorf("opening %s: %w", opts.Dir, err)
	}
	if err := ensureCloneable(opts.Dir); err != nil {
		return nil, err
	}
	repo, err = git.PlainCloneContext(ctx, opts.Dir, false, &git.CloneOptions{
		URL:           opts.URL,
		Auth:          opts.auth(),
		ReferenceName: plumbing.NewBranchReferenceName(opts.Branch),
		SingleBranch:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("cloning %s (branch %s) into %s: %w", opts.URL, opts.Branch, opts.Dir, err)
	}
	return repo, nil
}

// ensureCloneable fails on a non-empty directory rather than cloning into it,
// so a misconfigured --repo cannot scribble over unrelated files.
func ensureCloneable(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return os.MkdirAll(dir, 0o755)
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s is not empty and is not a git repository; refusing to clone into it", dir)
	}
	return nil
}

// fetchBranch updates the remote-tracking ref and returns it.
func fetchBranch(ctx context.Context, repo *git.Repository, opts SyncOptions) (*plumbing.Reference, error) {
	spec := config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", opts.Branch, opts.Branch))
	err := repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		Auth:       opts.auth(),
		RefSpecs:   []config.RefSpec{spec},
		Force:      true,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil, fmt.Errorf("fetching %s from %s: %w", opts.Branch, opts.URL, err)
	}
	ref, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", opts.Branch), true)
	if err != nil {
		return nil, fmt.Errorf("resolving origin/%s: %w", opts.Branch, err)
	}
	return ref, nil
}

// checkClean rejects modifications to tracked files. Untracked files are
// ignored: a hard reset leaves them alone, and the history database or build
// output may sit beside the manifests.
func checkClean(wt *git.Worktree) error {
	status, err := wt.Status()
	if err != nil {
		return err
	}
	for path, st := range status {
		if st.Worktree == git.Untracked {
			continue
		}
		if st.Worktree == git.Unmodified && st.Staging == git.Unmodified {
			continue
		}
		return fmt.Errorf("%w: %s", ErrLocalDirty, path)
	}
	return nil
}

// checkFastForward verifies local is an ancestor of remote, i.e. that moving
// to remote discards no local commit.
func checkFastForward(repo *git.Repository, local, remote plumbing.Hash) error {
	localCommit, err := repo.CommitObject(local)
	if err != nil {
		return fmt.Errorf("reading local commit %s: %w", local.String()[:7], err)
	}
	remoteCommit, err := repo.CommitObject(remote)
	if err != nil {
		return fmt.Errorf("reading remote commit %s: %w", remote.String()[:7], err)
	}
	ancestor, err := localCommit.IsAncestor(remoteCommit)
	if err != nil {
		return err
	}
	if !ancestor {
		return fmt.Errorf("%w: local %s is not an ancestor of origin %s; push or reset the working copy",
			ErrLocalAhead, local.String()[:7], remote.String()[:7])
	}
	return nil
}

// Push sends the current branch to origin. A promotion commits into the
// server's working copy, and Sync refuses to fast-forward past an unpushed
// commit, so serving from a remote means pushing promotions as they are made.
func Push(ctx context.Context, opts SyncOptions) error {
	repo, err := git.PlainOpenWithOptions(opts.Dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("opening git repository from %s: %w", opts.Dir, err)
	}
	spec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", opts.Branch, opts.Branch))
	err = repo.PushContext(ctx, &git.PushOptions{
		RemoteName: "origin",
		Auth:       opts.auth(),
		RefSpecs:   []config.RefSpec{spec},
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("pushing %s to origin: %w", opts.Branch, err)
	}
	return nil
}

// WorkDir returns the absolute working copy path, for callers that must hand
// a concrete directory to the manifest loader.
func (o SyncOptions) WorkDir() (string, error) {
	return filepath.Abs(o.Dir)
}

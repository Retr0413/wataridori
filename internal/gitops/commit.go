package gitops

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Commit stages the given files (absolute paths inside a git worktree) and
// commits them with message. It fails when tracked files outside the target
// set have modifications, so a promote never sweeps up unrelated edits;
// untracked files are ignored because staging is explicit.
func Commit(startDir string, files []string, message string) (commitID string, err error) {
	repo, err := git.PlainOpenWithOptions(startDir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return "", fmt.Errorf("opening git repository from %s: %w", startDir, err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}

	targets, err := worktreeRelative(wt.Filesystem.Root(), files)
	if err != nil {
		return "", err
	}

	status, err := wt.Status()
	if err != nil {
		return "", err
	}
	var unrelated []string
	for path, st := range status {
		if st.Worktree == git.Untracked {
			continue
		}
		if st.Worktree == git.Unmodified && st.Staging == git.Unmodified {
			continue
		}
		if !targets[path] {
			unrelated = append(unrelated, path)
		}
	}
	if len(unrelated) > 0 {
		sort.Strings(unrelated)
		return "", fmt.Errorf("worktree has unrelated changes, commit or stash them first: %s", strings.Join(unrelated, ", "))
	}

	changed := false
	for path := range targets {
		// Clean files are absent from the status map entirely.
		st, ok := status[path]
		if !ok || (st.Worktree == git.Unmodified && st.Staging == git.Unmodified) {
			continue
		}
		if _, err := wt.Add(path); err != nil {
			return "", fmt.Errorf("staging %s: %w", path, err)
		}
		changed = true
	}
	if !changed {
		return "", fmt.Errorf("nothing to commit: %s unchanged", strings.Join(sortedKeys(targets), ", "))
	}

	sig, err := signature(repo)
	if err != nil {
		return "", err
	}
	hash, err := wt.Commit(message, &git.CommitOptions{Author: sig})
	if err != nil {
		return "", err
	}
	return hash.String(), nil
}

func worktreeRelative(root string, files []string) (map[string]bool, error) {
	// The worktree root may be behind a symlink (e.g. /tmp on macOS).
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	targets := make(map[string]bool, len(files))
	for _, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			return nil, err
		}
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			abs = resolved
		}
		rel, err := filepath.Rel(resolvedRoot, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("%s is outside the git worktree %s", f, root)
		}
		targets[filepath.ToSlash(rel)] = true
	}
	return targets, nil
}

func signature(repo *git.Repository) (*object.Signature, error) {
	cfg, err := repo.ConfigScoped(config.GlobalScope) // local merged with ~/.gitconfig
	if err != nil {
		return nil, err
	}
	if cfg.User.Name == "" || cfg.User.Email == "" {
		return nil, fmt.Errorf("git user.name / user.email not configured; run git config --global user.name/user.email")
	}
	return &object.Signature{Name: cfg.User.Name, Email: cfg.User.Email, When: time.Now()}, nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

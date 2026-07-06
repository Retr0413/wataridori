package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// initRepo creates a git repo with user config and one initial commit
// containing the given files.
func initRepo(t *testing.T, files map[string]string) (string, *git.Repository) {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := repo.Config()
	if err != nil {
		t.Fatal(err)
	}
	cfg.User.Name, cfg.User.Email = "Test User", "test@example.com"
	if err := repo.SetConfig(cfg); err != nil {
		t.Fatal(err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		writeFile(t, dir, name, content)
		if _, err := wt.Add(name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "Test User", Email: "test@example.com"},
	}); err != nil {
		t.Fatal(err)
	}
	return dir, repo
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCommit(t *testing.T) {
	dir, repo := initRepo(t, map[string]string{"envs/prod/app.yaml": "image: old\n"})
	writeFile(t, dir, "envs/prod/app.yaml", "image: new\n")

	msg := PromoteMessage("prod", "app", "abc123def456", "dev")
	id, err := Commit(dir, []string{filepath.Join(dir, "envs/prod/app.yaml")}, msg)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	if head.Hash().String() != id {
		t.Errorf("returned id %s != HEAD %s", id, head.Hash())
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	if commit.Message != "promote(prod): app to abc123def456 (from dev)" {
		t.Errorf("message = %q", commit.Message)
	}
	if commit.Author.Email != "test@example.com" {
		t.Errorf("author = %+v", commit.Author)
	}

	// Worktree must be clean afterwards.
	wt, _ := repo.Worktree()
	status, _ := wt.Status()
	if !status.IsClean() {
		t.Errorf("worktree dirty after commit: %v", status)
	}
}

func TestCommitRejectsUnrelatedChanges(t *testing.T) {
	dir, _ := initRepo(t, map[string]string{
		"envs/prod/app.yaml": "image: old\n",
		"README.md":          "hello\n",
	})
	writeFile(t, dir, "envs/prod/app.yaml", "image: new\n")
	writeFile(t, dir, "README.md", "edited\n")

	_, err := Commit(dir, []string{filepath.Join(dir, "envs/prod/app.yaml")}, "msg")
	if err == nil || !strings.Contains(err.Error(), "README.md") {
		t.Errorf("want unrelated-changes error naming README.md, got %v", err)
	}
}

func TestCommitAllowsUntrackedFiles(t *testing.T) {
	dir, _ := initRepo(t, map[string]string{"envs/prod/app.yaml": "image: old\n"})
	writeFile(t, dir, "envs/prod/app.yaml", "image: new\n")
	writeFile(t, dir, "scratch.txt", "untracked\n")

	if _, err := Commit(dir, []string{filepath.Join(dir, "envs/prod/app.yaml")}, "msg"); err != nil {
		t.Errorf("untracked file should not block commit: %v", err)
	}
}

func TestCommitNothingToCommit(t *testing.T) {
	dir, _ := initRepo(t, map[string]string{"envs/prod/app.yaml": "image: old\n"})
	_, err := Commit(dir, []string{filepath.Join(dir, "envs/prod/app.yaml")}, "msg")
	if err == nil || !strings.Contains(err.Error(), "nothing to commit") {
		t.Errorf("want nothing-to-commit error, got %v", err)
	}
}

func TestCommitFileOutsideWorktree(t *testing.T) {
	dir, _ := initRepo(t, map[string]string{"a.txt": "x\n"})
	outside := filepath.Join(t.TempDir(), "b.txt")
	if err := os.WriteFile(outside, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Commit(dir, []string{outside}, "msg"); err == nil {
		t.Error("file outside worktree: want error")
	}
}

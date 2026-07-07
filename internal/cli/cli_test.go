package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const (
	digestOld = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestNew = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// setupRepo creates a committed manifest repository with dev ahead of prod.
func setupRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"wataridori.yaml": `version: 1
environments:
  dev:
    policy: auto
    branch: develop
    gcp: {project: p-dev, region: asia-northeast1}
    services: envs/dev
  prod:
    policy: manual
    promoteFrom: dev
    gcp: {project: p-prod, region: asia-northeast1}
    services: envs/prod
`,
		"envs/dev/my-app.yaml":  "name: my-app\nimage: reg.example/app/my-app@" + digestNew + "\n",
		"envs/prod/my-app.yaml": "name: my-app\nimage: reg.example/app/my-app@" + digestOld + "\n",
	}
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := repo.Config()
	cfg.User.Name, cfg.User.Email = "Test", "test@example.com"
	if err := repo.SetConfig(cfg); err != nil {
		t.Fatal(err)
	}
	wt, _ := repo.Worktree()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Add(name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com"},
	}); err != nil {
		t.Fatal(err)
	}
	return dir
}

// run executes the CLI with args and returns combined output.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return buf.String(), err
}

// TestPromoteEndToEnd covers the full local promote flow: manifest rewrite,
// git commit, history record. No network involved (no imageCopy).
func TestPromoteEndToEnd(t *testing.T) {
	dir := setupRepo(t)
	db := filepath.Join(t.TempDir(), "history.db")

	out, err := run(t, "promote", "--to", "prod", "--yes", "--repo", dir, "--db", db)
	if err != nil {
		t.Fatalf("promote: %v\n%s", err, out)
	}
	if !strings.Contains(out, "committed") {
		t.Errorf("output = %q", out)
	}

	// Manifest rewritten with dev's digest, image path preserved.
	data, _ := os.ReadFile(filepath.Join(dir, "envs/prod/my-app.yaml"))
	if !strings.Contains(string(data), "reg.example/app/my-app@"+digestNew) {
		t.Errorf("prod manifest not rewritten:\n%s", data)
	}

	// Commit exists with the conventional message.
	repo, _ := git.PlainOpen(dir)
	head, _ := repo.Head()
	commit, _ := repo.CommitObject(head.Hash())
	if !strings.HasPrefix(commit.Message, "promote(prod): my-app to bbbbbbbbbbbb (from dev)") {
		t.Errorf("commit message = %q", commit.Message)
	}

	// History records the promote.
	histOut, err := run(t, "history", "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(histOut, "promote") || !strings.Contains(histOut, "prod") ||
		!strings.Contains(histOut, "bbbbbbbbbbbb") {
		t.Errorf("history output = %q", histOut)
	}

	// Second promote is a no-op.
	out, err = run(t, "promote", "--to", "prod", "--yes", "--repo", dir, "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "nothing to promote") {
		t.Errorf("second promote output = %q", out)
	}
}

func TestPromoteDeclined(t *testing.T) {
	dir := setupRepo(t)
	db := filepath.Join(t.TempDir(), "history.db")

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader("n\n"))
	root.SetArgs([]string{"promote", "--to", "prod", "--repo", dir, "--db", db})
	err := root.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Errorf("want aborted error, got %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "envs/prod/my-app.yaml"))
	if strings.Contains(string(data), digestNew) {
		t.Error("declined promote must not rewrite the manifest")
	}
}

func TestVersion(t *testing.T) {
	out, err := run(t, "version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "wataridori dev") {
		t.Errorf("version output = %q", out)
	}
}

func TestShortImage(t *testing.T) {
	got := shortImage("reg.example/app/my-app@" + digestOld)
	if got != "my-app@aaaaaaaaaaaa" {
		t.Errorf("shortImage = %q", got)
	}
	if shortImage("") != "-" {
		t.Error("empty image should render as dash")
	}
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestValidateJSONLoadsEveryManifestWithoutMutation(t *testing.T) {
	dir := setupRepo(t)
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	before, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "validate", "--repo", dir, "--json")
	if err != nil {
		t.Fatalf("validate: %v\n%s", err, out)
	}
	var result struct {
		Environments int `json:"environments"`
		Services     int `json:"services"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("JSON output: %v\n%s", err, out)
	}
	if result.Environments != 2 || result.Services != 2 {
		t.Errorf("result = %+v", result)
	}
	after, _ := repo.Head()
	if before.Hash() != after.Hash() {
		t.Error("validate must not create a commit")
	}
}

func TestManifestSetImageUpdatesOneFileAndIsIdempotent(t *testing.T) {
	dir := setupRepo(t)
	image := "reg.example/app/my-app@sha256:" + strings.Repeat("c", 64)

	out, err := run(t, "manifest", "set-image", "--env", "dev", "--service", "my-app", "--image", image, "--repo", dir, "--json")
	if err != nil {
		t.Fatalf("set-image: %v\n%s", err, out)
	}
	var result struct {
		File    string `json:"file"`
		Changed bool   `json:"changed"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.File != filepath.Join("envs", "dev", "my-app.yaml") {
		t.Errorf("result = %+v", result)
	}
	devData, _ := os.ReadFile(filepath.Join(dir, "envs/dev/my-app.yaml"))
	prodData, _ := os.ReadFile(filepath.Join(dir, "envs/prod/my-app.yaml"))
	if !strings.Contains(string(devData), image) {
		t.Errorf("dev manifest = %s", devData)
	}
	if strings.Contains(string(prodData), strings.Repeat("c", 64)) {
		t.Error("set-image must not change another environment")
	}

	out, err = run(t, "manifest", "set-image", "--env", "dev", "--service", "my-app", "--image", image, "--repo", dir, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("second set-image should be a no-op")
	}
}

func TestManifestSetImageRejectsTagWithoutWriting(t *testing.T) {
	dir := setupRepo(t)
	path := filepath.Join(dir, "envs/dev/my-app.yaml")
	before, _ := os.ReadFile(path)

	_, err := run(t, "manifest", "set-image", "--env", "dev", "--service", "my-app", "--image", "reg.example/app/my-app:latest", "--repo", dir)
	if err == nil || !strings.Contains(err.Error(), "not digest-pinned") {
		t.Fatalf("error = %v", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Error("invalid image must not modify the manifest")
	}
}

func TestManifestSetImageCanRequireAutoPolicy(t *testing.T) {
	dir := setupRepo(t)
	image := "reg.example/app/my-app@sha256:" + strings.Repeat("c", 64)
	_, err := run(t, "manifest", "set-image", "--env", "prod", "--service", "my-app", "--image", image, "--require-policy", "auto", "--repo", dir)
	if err == nil || !strings.Contains(err.Error(), `policy "auto" is required`) {
		t.Fatalf("error = %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "envs/prod/my-app.yaml"))
	if strings.Contains(string(data), strings.Repeat("c", 64)) {
		t.Error("policy mismatch must not modify prod")
	}
}

func TestPromoteDryRunJSONDoesNotMutate(t *testing.T) {
	dir := setupRepo(t)
	db := filepath.Join(t.TempDir(), "history.db")
	repo, _ := git.PlainOpen(dir)
	before, _ := repo.Head()

	out, err := run(t, "promote", "--to", "prod", "--dry-run", "--json", "--repo", dir, "--db", db)
	if err != nil {
		t.Fatalf("promote dry-run: %v\n%s", err, out)
	}
	var plan struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Items []any  `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.From != "dev" || plan.To != "prod" || len(plan.Items) != 1 {
		t.Errorf("plan = %+v", plan)
	}
	after, _ := repo.Head()
	if before.Hash() != after.Hash() {
		t.Error("dry-run must not commit")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "envs/prod/my-app.yaml"))
	if strings.Contains(string(data), digestNew) {
		t.Error("dry-run must not rewrite prod")
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Errorf("dry-run must not open history DB, stat error = %v", err)
	}
}

func TestResolveActorPrefersExplicitOverride(t *testing.T) {
	t.Setenv("WATARIDORI_ACTOR", "github:octocat")
	if got := resolveActor(); got != "github:octocat" {
		t.Errorf("resolveActor = %q", got)
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

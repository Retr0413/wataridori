package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Retr0413/wataridori/internal/cloudrun"
	"github.com/Retr0413/wataridori/internal/manifest"
	"github.com/Retr0413/wataridori/internal/store"
)

const (
	digestOld = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestNew = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

type fakeCloudRun struct {
	deployed   map[string]*cloudrun.Deployed // key: env/service
	revisions  map[string][]cloudrun.Revision
	applied    []string          // env/service actually deployed
	traffic    map[string]string // env/service -> pinned revision
	applyImage map[string]string // image recorded at Apply time
}

func newFakeCloudRun() *fakeCloudRun {
	return &fakeCloudRun{
		deployed:   map[string]*cloudrun.Deployed{},
		revisions:  map[string][]cloudrun.Revision{},
		traffic:    map[string]string{},
		applyImage: map[string]string{},
	}
}

func key(env *manifest.Environment, service string) string { return env.Name + "/" + service }

func (f *fakeCloudRun) Get(_ context.Context, env *manifest.Environment, name string) (*cloudrun.Deployed, error) {
	return f.deployed[key(env, name)], nil
}

func (f *fakeCloudRun) ListServices(_ context.Context, env *manifest.Environment) ([]cloudrun.Deployed, error) {
	var out []cloudrun.Deployed
	prefix := env.Name + "/"
	for k, deployed := range f.deployed {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		out = append(out, *deployed)
	}
	return out, nil
}

func (f *fakeCloudRun) Apply(_ context.Context, env *manifest.Environment, svc *manifest.Service, _ time.Duration) (*cloudrun.Deployed, error) {
	k := key(env, svc.Name)
	f.applied = append(f.applied, k)
	f.applyImage[k] = svc.Image
	d := &cloudrun.Deployed{Service: svc.Name, Image: svc.Image, Revision: svc.Name + "-00002", Ready: true, URL: "https://" + svc.Name + ".run.app"}
	f.deployed[k] = d
	return d, nil
}

func (f *fakeCloudRun) ListRevisions(_ context.Context, env *manifest.Environment, service string) ([]cloudrun.Revision, error) {
	return f.revisions[key(env, service)], nil
}

func (f *fakeCloudRun) SetTraffic(_ context.Context, env *manifest.Environment, service, revision string) error {
	f.traffic[key(env, service)] = revision
	return nil
}

type fakeCopier struct {
	calls []string // "src -> dst"
}

func (f *fakeCopier) Copy(_ context.Context, srcRef, dstPath string) (bool, error) {
	f.calls = append(f.calls, srcRef+" -> "+dstPath)
	return true, nil
}

type fakeCommitter struct {
	files   []string
	message string
	commits int
}

func (f *fakeCommitter) Commit(_ string, files []string, message string) (string, error) {
	f.files = files
	f.message = message
	f.commits++
	return "commit1234567", nil
}

type fakeHistory struct {
	entries []store.Entry
}

func (f *fakeHistory) Record(_ context.Context, e store.Entry) error {
	f.entries = append(f.entries, e)
	return nil
}

func (f *fakeHistory) List(_ context.Context, opts store.ListOptions) ([]store.Entry, error) {
	var out []store.Entry
	for i := len(f.entries) - 1; i >= 0; i-- {
		if opts.Env == "" || f.entries[i].Env == opts.Env {
			out = append(out, f.entries[i])
		}
	}
	return out, nil
}

// testRepo writes a two-environment manifest repository. When imageCopy is
// true, prod declares a separate registry (copy-on-promote setup).
func testRepo(t *testing.T, imageCopy bool) *manifest.Repo {
	t.Helper()
	copyBlock := ""
	if imageCopy {
		copyBlock = "\n    imageCopy:\n      to: reg.example/prod"
	}
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
    services: envs/prod` + copyBlock + "\n",
		"envs/dev/my-app.yaml":  "name: my-app\nimage: reg.example/dev/my-app@" + digestNew + "\n",
		"envs/prod/my-app.yaml": "name: my-app\nimage: reg.example/prod/my-app@" + digestOld + "\n",
	}
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo, warnings, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) > 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	return repo
}

type testEngine struct {
	*Engine
	cloudRun  *fakeCloudRun
	copier    *fakeCopier
	committer *fakeCommitter
	history   *fakeHistory
}

func newTestEngine(t *testing.T, imageCopy bool) *testEngine {
	t.Helper()
	cr := newFakeCloudRun()
	cp := &fakeCopier{}
	cm := &fakeCommitter{}
	h := &fakeHistory{}
	return &testEngine{
		Engine: &Engine{
			Repo:     testRepo(t, imageCopy),
			CloudRun: cr,
			Copier:   cp,
			Commit:   cm,
			History:  h,
			Actor:    "tester@example.com",
		},
		cloudRun: cr, copier: cp, committer: cm, history: h,
	}
}

func mustContain(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want containing %q", err, want)
	}
}

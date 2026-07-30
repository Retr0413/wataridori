package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Retr0413/wataridori/internal/cloudrun"
	"github.com/Retr0413/wataridori/internal/manifest"
)

// splitNameRepo is the shape this feature exists for: one project, one logical
// service, and a Cloud Run name that encodes the environment. Without
// cloudRunName the two environments share no name to promote along.
func splitNameRepo(t *testing.T) *manifest.Repo {
	t.Helper()
	files := map[string]string{
		"wataridori.yaml": `version: 1
environments:
  dev:
    policy: manual
    gcp: {project: shared, region: asia-northeast1}
    services: envs/dev
  prod:
    policy: manual
    promoteFrom: dev
    gcp: {project: shared, region: asia-northeast1}
    services: envs/prod
`,
		"envs/dev/api.yaml":  "name: api\ncloudRunName: api-dev\nimage: reg.example/api@" + digestNew + "\n",
		"envs/prod/api.yaml": "name: api\ncloudRunName: api-prod\nimage: reg.example/api@" + digestOld + "\n",
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
	repo, _, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func splitNameEngine(t *testing.T) *testEngine {
	t.Helper()
	e := newTestEngine(t, false)
	e.Repo = splitNameRepo(t)
	return e
}

func TestPromoteMatchesAcrossDifferentCloudRunNames(t *testing.T) {
	e := splitNameEngine(t)

	plan, err := e.PlanPromote(context.Background(), PromoteRequest{To: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(plan.Items))
	}
	item := plan.Items[0]
	if item.Service != "api" {
		t.Errorf("service = %q, want api", item.Service)
	}
	// The digest moves; the target keeps its own image path.
	if _, digest, _ := manifest.SplitDigest(item.NewImage); digest != digestNew {
		t.Errorf("new digest = %q, want %q", digest, digestNew)
	}
}

func TestStatusReadsTheCloudRunNameNotTheManifestName(t *testing.T) {
	e := splitNameEngine(t)
	e.cloudRun.deployed["prod/api-prod"] = &cloudrun.Deployed{
		Service: "api-prod", Image: "reg.example/api@" + digestOld,
		Revision: "api-prod-00007", Ready: true, TrafficPercent: 100,
	}

	res, err := e.Status(context.Background(), StatusRequest{Env: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	got := res.Services[0]
	if got.State != StateInSync {
		t.Fatalf("state = %q, want in sync (looked up under the wrong name?)", got.State)
	}
	// The row is identified by the shared name so both environments line up,
	// while the Cloud Run name stays available for links and messages.
	if got.Service != "api" || got.RunName != "api-prod" {
		t.Errorf("service/runName = %q/%q, want api/api-prod", got.Service, got.RunName)
	}
}

func TestInventoryMatchesManagedServicesByCloudRunName(t *testing.T) {
	e := splitNameEngine(t)
	e.cloudRun.deployed["dev/api-dev"] = &cloudrun.Deployed{
		Service: "api-dev", Image: "reg.example/api@" + digestNew, Revision: "api-dev-00003",
	}

	res, err := e.Inventory(context.Background(), InventoryRequest{Env: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(res.Items))
	}
	item := res.Items[0]
	if !item.Managed {
		t.Error("managed = false; the manifest declares this service under cloudRunName")
	}
	if item.Service != "api-dev" || item.ManifestService != "api" {
		t.Errorf("service/manifestService = %q/%q, want api-dev/api", item.Service, item.ManifestService)
	}
}

func TestRollbackSwitchesTrafficOnTheCloudRunService(t *testing.T) {
	e := splitNameEngine(t)
	e.cloudRun.revisions["prod/api-prod"] = []cloudrun.Revision{
		{Name: "api-prod-00002", Image: "reg.example/api@" + digestNew, Ready: true, TrafficPercent: 100},
		{Name: "api-prod-00001", Image: "reg.example/api@" + digestOld, Ready: true},
	}

	plan, err := e.PlanRollback(context.Background(), RollbackRequest{Env: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.ExecuteRollback(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if got := e.cloudRun.traffic["prod/api-prod"]; got != "api-prod-00001" {
		t.Errorf("traffic pinned to %q under key prod/api-prod, want api-prod-00001", got)
	}
}

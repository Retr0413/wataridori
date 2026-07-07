package core

import (
	"context"
	"testing"
	"time"

	"github.com/Retr0413/wataridori/internal/cloudrun"
	"github.com/Retr0413/wataridori/internal/manifest"
	"github.com/Retr0413/wataridori/internal/store"
)

var ctx = context.Background()

// --- apply ---

func TestApplyDeploysAndRecords(t *testing.T) {
	e := newTestEngine(t, false)
	res, err := e.Apply(ctx, ApplyRequest{Env: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Services) != 1 || !res.Services[0].InSync || res.Services[0].Revision != "my-app-00002" {
		t.Errorf("result = %+v", res.Services)
	}
	if len(e.cloudRun.applied) != 1 || e.cloudRun.applied[0] != "dev/my-app" {
		t.Errorf("applied = %v", e.cloudRun.applied)
	}
	if len(e.history.entries) != 1 {
		t.Fatalf("history = %+v", e.history.entries)
	}
	entry := e.history.entries[0]
	if entry.Action != store.ActionApply || entry.Env != "dev" || entry.Digest != digestNew ||
		entry.Actor != "tester@example.com" || entry.Detail["revision"] != "my-app-00002" {
		t.Errorf("entry = %+v", entry)
	}
}

func TestApplyDryRunDoesNotDeploy(t *testing.T) {
	e := newTestEngine(t, false)
	e.cloudRun.deployed["dev/my-app"] = &cloudrun.Deployed{Service: "my-app", Image: "reg.example/dev/my-app@" + digestOld}

	res, err := e.Apply(ctx, ApplyRequest{Env: "dev", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.cloudRun.applied) != 0 {
		t.Error("dry-run must not deploy")
	}
	if len(e.history.entries) != 0 {
		t.Error("dry-run must not record history")
	}
	if res.Services[0].InSync {
		t.Error("digest differs; want InSync=false")
	}
	if res.Services[0].ActualImage == "" {
		t.Error("dry-run should report the current image")
	}
}

func TestApplyUnknownServiceFilter(t *testing.T) {
	e := newTestEngine(t, false)
	_, err := e.Apply(ctx, ApplyRequest{Env: "dev", Service: "ghost"})
	mustContain(t, err, `no service named "ghost"`)
}

// --- promote ---

func TestPlanPromoteDiff(t *testing.T) {
	e := newTestEngine(t, false)
	plan, err := e.PlanPromote(ctx, PromoteRequest{To: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.From != "dev" || plan.To != "prod" || len(plan.Items) != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	item := plan.Items[0]
	// Digest moves; the image path stays prod's (spec §1.3).
	if item.NewImage != "reg.example/prod/my-app@"+digestNew {
		t.Errorf("NewImage = %s", item.NewImage)
	}
	if item.FromImage != "reg.example/dev/my-app@"+digestNew {
		t.Errorf("FromImage = %s", item.FromImage)
	}
	if item.NeedsCopy {
		t.Error("no imageCopy configured; NeedsCopy should be false")
	}
}

func TestPlanPromoteNoDiffIsEmpty(t *testing.T) {
	e := newTestEngine(t, false)
	env, _ := e.Repo.Environment("prod")
	services, _ := e.Repo.LoadServices(env)
	if err := e.Repo.UpdateServiceImage(services[0], "reg.example/prod/my-app@"+digestNew); err != nil {
		t.Fatal(err)
	}

	plan, err := e.PlanPromote(ctx, PromoteRequest{To: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 0 {
		t.Errorf("want empty plan, got %+v", plan.Items)
	}
	// Executing an empty plan is a no-op.
	res, err := e.ExecutePromote(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	if res.CommitID != "" || e.committer.commits != 0 {
		t.Error("empty plan must not commit")
	}
}

func TestPlanPromoteRejectsAutoTarget(t *testing.T) {
	e := newTestEngine(t, false)
	_, err := e.PlanPromote(ctx, PromoteRequest{To: "dev", From: "prod"})
	mustContain(t, err, `policy "auto"`)
}

func TestPlanPromoteSelfReference(t *testing.T) {
	e := newTestEngine(t, false)
	_, err := e.PlanPromote(ctx, PromoteRequest{To: "prod", From: "prod"})
	mustContain(t, err, "into itself")
}

func TestExecutePromoteWithoutCopy(t *testing.T) {
	e := newTestEngine(t, false)
	plan, err := e.PlanPromote(ctx, PromoteRequest{To: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.ExecutePromote(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}

	if len(e.copier.calls) != 0 {
		t.Errorf("no imageCopy configured, but copier called: %v", e.copier.calls)
	}
	if res.CommitID != "commit1234567" || e.committer.commits != 1 {
		t.Errorf("commit = %+v", e.committer)
	}
	if e.committer.message != "promote(prod): my-app to bbbbbbbbbbbb (from dev)" {
		t.Errorf("message = %q", e.committer.message)
	}

	// Manifest file rewritten with the new digest.
	env, _ := e.Repo.Environment("prod")
	services, err := e.Repo.LoadServices(env)
	if err != nil {
		t.Fatal(err)
	}
	if services[0].Image != "reg.example/prod/my-app@"+digestNew {
		t.Errorf("manifest not rewritten: %s", services[0].Image)
	}

	entry := e.history.entries[0]
	if entry.Action != store.ActionPromote || entry.Env != "prod" || entry.Digest != digestNew ||
		entry.Detail["from"] != "dev" || entry.Detail["commit"] != "commit1234567" {
		t.Errorf("entry = %+v", entry)
	}
}

func TestExecutePromoteCopiesWhenConfigured(t *testing.T) {
	e := newTestEngine(t, true)
	plan, err := e.PlanPromote(ctx, PromoteRequest{To: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Items[0].NeedsCopy {
		t.Fatal("want NeedsCopy=true")
	}
	if _, err := e.ExecutePromote(ctx, plan); err != nil {
		t.Fatal(err)
	}
	want := "reg.example/dev/my-app@" + digestNew + " -> reg.example/prod/my-app"
	if len(e.copier.calls) != 1 || e.copier.calls[0] != want {
		t.Errorf("copier calls = %v, want [%s]", e.copier.calls, want)
	}
}

// --- rollback ---

func revisionsFixture() []cloudrun.Revision {
	now := time.Now()
	return []cloudrun.Revision{
		{Name: "my-app-00003", Image: "img@" + digestNew, CreateTime: now, Ready: true, TrafficPercent: 100},
		{Name: "my-app-00002", Image: "img@sha256:broken", CreateTime: now.Add(-time.Hour), Ready: false},
		{Name: "my-app-00001", Image: "img@" + digestOld, CreateTime: now.Add(-2 * time.Hour), Ready: true},
	}
}

func TestPlanRollbackSkipsNotReady(t *testing.T) {
	e := newTestEngine(t, false)
	e.cloudRun.revisions["prod/my-app"] = revisionsFixture()

	plan, err := e.PlanRollback(ctx, RollbackRequest{Env: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	item := plan.Items[0]
	// 00002 is not ready and must be skipped in favour of 00001.
	if item.CurrentRevision != "my-app-00003" || item.TargetRevision != "my-app-00001" {
		t.Errorf("item = %+v", item)
	}
}

func TestPlanRollbackNoTarget(t *testing.T) {
	e := newTestEngine(t, false)
	e.cloudRun.revisions["prod/my-app"] = []cloudrun.Revision{
		{Name: "my-app-00001", Ready: true, TrafficPercent: 100},
	}
	_, err := e.PlanRollback(ctx, RollbackRequest{Env: "prod"})
	mustContain(t, err, "no ready revision older than")
}

func TestPlanRollbackExplicitRevision(t *testing.T) {
	e := newTestEngine(t, false)
	e.cloudRun.revisions["prod/my-app"] = revisionsFixture()

	plan, err := e.PlanRollback(ctx, RollbackRequest{Env: "prod", Service: "my-app", Revision: "my-app-00001"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Items[0].TargetRevision != "my-app-00001" {
		t.Errorf("item = %+v", plan.Items[0])
	}

	_, err = e.PlanRollback(ctx, RollbackRequest{Env: "prod", Service: "my-app", Revision: "my-app-00002"})
	mustContain(t, err, "not ready")

	_, err = e.PlanRollback(ctx, RollbackRequest{Env: "prod", Service: "my-app", Revision: "ghost"})
	mustContain(t, err, "no revision named")
}

func TestExecuteRollback(t *testing.T) {
	e := newTestEngine(t, false)
	e.cloudRun.revisions["prod/my-app"] = revisionsFixture()

	plan, err := e.PlanRollback(ctx, RollbackRequest{Env: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.ExecuteRollback(ctx, plan); err != nil {
		t.Fatal(err)
	}
	if e.cloudRun.traffic["prod/my-app"] != "my-app-00001" {
		t.Errorf("traffic = %v", e.cloudRun.traffic)
	}
	entry := e.history.entries[0]
	if entry.Action != store.ActionRollback || entry.Digest != digestOld ||
		entry.Detail["rolledBackFrom"] != "my-app-00003" {
		t.Errorf("entry = %+v", entry)
	}
}

// --- status ---

func TestStatusStates(t *testing.T) {
	e := newTestEngine(t, false)
	// dev: in sync; prod: drift (actual runs the old digest? no — manifest
	// has old digest, Cloud Run serves the new one, e.g. after a rollback
	// in the opposite direction).
	e.cloudRun.deployed["dev/my-app"] = &cloudrun.Deployed{
		Service: "my-app", Image: "reg.example/dev/my-app@" + digestNew,
		Revision: "my-app-00042", Ready: true, TrafficPercent: 100,
	}
	e.cloudRun.deployed["prod/my-app"] = &cloudrun.Deployed{
		Service: "my-app", Image: "reg.example/prod/my-app@" + digestNew,
		Revision: "my-app-00017", Ready: true, TrafficPercent: 100,
	}

	res, err := e.Status(ctx, StatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Services) != 2 {
		t.Fatalf("services = %+v", res.Services)
	}
	byEnv := map[string]ServiceStatus{}
	for _, s := range res.Services {
		byEnv[s.Env] = s
	}
	if byEnv["dev"].State != StateInSync {
		t.Errorf("dev = %+v", byEnv["dev"])
	}
	if byEnv["prod"].State != StateDrift {
		t.Errorf("prod = %+v", byEnv["prod"])
	}
	if !res.Drift {
		t.Error("want Drift=true")
	}
}

func TestStatusNotDeployed(t *testing.T) {
	e := newTestEngine(t, false)
	res, err := e.Status(ctx, StatusRequest{Env: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Services[0].State != StateNotDeployed || !res.Drift {
		t.Errorf("result = %+v", res)
	}
}

// --- history ---

func TestListHistory(t *testing.T) {
	e := newTestEngine(t, false)
	if _, err := e.Apply(ctx, ApplyRequest{Env: "dev"}); err != nil {
		t.Fatal(err)
	}
	res, err := e.ListHistory(ctx, HistoryRequest{Env: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 1 || res.Entries[0].Action != store.ActionApply {
		t.Errorf("entries = %+v", res.Entries)
	}
}

// guard against accidental interface drift between core and the real
// implementations (compile-time only).
var (
	_ ImageCopier = (interface {
		Copy(ctx context.Context, srcRef, dstPath string) (bool, error)
	})(nil)
	_ = manifest.WithDigest
)

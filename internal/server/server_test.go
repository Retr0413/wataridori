package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/Retr0413/wataridori/gen/wataridori/v1"
	"github.com/Retr0413/wataridori/internal/core"
	"github.com/Retr0413/wataridori/internal/store"
)

// fakeUseCases records the requests it receives and returns canned results.
type fakeUseCases struct {
	envsRes     *core.ListEnvironmentsResult
	statusReq   core.StatusRequest
	statusRes   *core.StatusResult
	invReq      core.InventoryRequest
	invRes      *core.InventoryResult
	applyReq    core.ApplyRequest
	applyRes    *core.ApplyResult
	promoteReq  core.PromoteRequest
	promotePlan *core.PromotePlan
	promoteRes  *core.PromoteResult
	promoteErr  error
	historyRes  *core.HistoryResult
	timelineReq core.TimelineRequest
	timelineRes *core.TimelineResult

	planPromoteCalls, execPromoteCalls int
}

func (f *fakeUseCases) ListEnvironments(_ context.Context, _ core.ListEnvironmentsRequest) (*core.ListEnvironmentsResult, error) {
	return f.envsRes, nil
}

func (f *fakeUseCases) Status(_ context.Context, req core.StatusRequest) (*core.StatusResult, error) {
	f.statusReq = req
	return f.statusRes, nil
}

func (f *fakeUseCases) Inventory(_ context.Context, req core.InventoryRequest) (*core.InventoryResult, error) {
	f.invReq = req
	return f.invRes, nil
}

func (f *fakeUseCases) Apply(_ context.Context, req core.ApplyRequest) (*core.ApplyResult, error) {
	f.applyReq = req
	return f.applyRes, nil
}

func (f *fakeUseCases) PlanPromote(_ context.Context, req core.PromoteRequest) (*core.PromotePlan, error) {
	f.planPromoteCalls++
	f.promoteReq = req
	if f.promoteErr != nil {
		return nil, f.promoteErr
	}
	return f.promotePlan, nil
}

func (f *fakeUseCases) ExecutePromote(_ context.Context, _ *core.PromotePlan) (*core.PromoteResult, error) {
	f.execPromoteCalls++
	return f.promoteRes, nil
}

func (f *fakeUseCases) PlanRollback(_ context.Context, _ core.RollbackRequest) (*core.RollbackPlan, error) {
	return &core.RollbackPlan{}, nil
}

func (f *fakeUseCases) ExecuteRollback(_ context.Context, _ *core.RollbackPlan) (*core.RollbackResult, error) {
	return &core.RollbackResult{}, nil
}

func (f *fakeUseCases) ListHistory(_ context.Context, _ core.HistoryRequest) (*core.HistoryResult, error) {
	return f.historyRes, nil
}

func (f *fakeUseCases) Timeline(_ context.Context, req core.TimelineRequest) (*core.TimelineResult, error) {
	f.timelineReq = req
	return f.timelineRes, nil
}

// srv wires the fake behind a Server, tracking cleanup invocation.
func srv(f *fakeUseCases, cleaned *bool) *Server {
	return New(func(context.Context) (UseCases, func(), error) {
		return f, func() { *cleaned = true }, nil
	})
}

func TestStatus(t *testing.T) {
	f := &fakeUseCases{statusRes: &core.StatusResult{
		Drift: true,
		Services: []core.ServiceStatus{{
			Env: "prod", Service: "api", DesiredDigest: "sha256:abc",
			State: core.StateDrift, TrafficPct: 100,
		}},
	}}
	cleaned := false
	res, err := srv(f, &cleaned).Status(context.Background(),
		connect.NewRequest(&v1.StatusRequest{Env: "prod"}))
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if f.statusReq.Env != "prod" {
		t.Errorf("env not forwarded: %q", f.statusReq.Env)
	}
	if !cleaned {
		t.Error("cleanup not called")
	}
	if !res.Msg.GetDrift() || len(res.Msg.GetServices()) != 1 {
		t.Fatalf("unexpected response: %+v", res.Msg)
	}
	got := res.Msg.GetServices()[0]
	if got.GetState() != v1.SyncState_SYNC_STATE_DRIFT {
		t.Errorf("state = %v, want DRIFT", got.GetState())
	}
	if got.GetTrafficPercent() != 100 {
		t.Errorf("traffic = %d, want 100", got.GetTrafficPercent())
	}
}

func TestStatusConnectJSON(t *testing.T) {
	f := &fakeUseCases{statusRes: &core.StatusResult{
		Drift: true,
		Services: []core.ServiceStatus{{
			Env: "prod", Service: "api", DesiredImage: "repo/api@sha256:abc",
			ActualImage: "repo/api@sha256:def", State: core.StateDrift,
		}},
	}}
	cleaned := false
	path, handler := srv(f, &cleaned).Handler()
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := bytes.NewBufferString(`{"env":"prod"}`)
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wataridori.v1.DeploymentService/Status", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}

	var msg struct {
		Drift    bool `json:"drift"`
		Services []struct {
			Env     string `json:"env"`
			Service string `json:"service"`
			State   string `json:"state"`
		} `json:"services"`
	}
	if err := json.NewDecoder(res.Body).Decode(&msg); err != nil {
		t.Fatal(err)
	}
	if !msg.Drift || len(msg.Services) != 1 {
		t.Fatalf("unexpected response: %+v", msg)
	}
	if msg.Services[0].State != "SYNC_STATE_DRIFT" {
		t.Errorf("state = %q, want SYNC_STATE_DRIFT", msg.Services[0].State)
	}
	if f.statusReq.Env != "prod" {
		t.Errorf("env not forwarded: %q", f.statusReq.Env)
	}
	if !cleaned {
		t.Error("cleanup not called")
	}
}

func TestInventory(t *testing.T) {
	f := &fakeUseCases{invRes: &core.InventoryResult{
		Items: []core.InventoryItem{{
			Env: "prod", Project: "p", Region: "asia-northeast1", Service: "api",
			Managed: true, DesiredDigest: "sha256:abc", ActualDigest: "sha256:def",
			State: core.InventoryDrift, TrafficPct: 100,
		}},
	}}
	cleaned := false
	res, err := srv(f, &cleaned).Inventory(context.Background(),
		connect.NewRequest(&v1.InventoryRequest{Env: "prod"}))
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if f.invReq.Env != "prod" {
		t.Errorf("env not forwarded: %q", f.invReq.Env)
	}
	if !cleaned {
		t.Error("cleanup not called")
	}
	items := res.Msg.GetItems()
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	got := items[0]
	if got.GetState() != v1.InventoryState_INVENTORY_STATE_DRIFT {
		t.Errorf("state = %v, want DRIFT", got.GetState())
	}
	if !got.GetManaged() || got.GetTrafficPercent() != 100 {
		t.Errorf("item = %+v", got)
	}
}

func TestApplyTimeoutDefault(t *testing.T) {
	f := &fakeUseCases{applyRes: &core.ApplyResult{Env: "dev"}}
	cleaned := false
	// timeout_seconds omitted -> server fills the default.
	_, err := srv(f, &cleaned).Apply(context.Background(),
		connect.NewRequest(&v1.ApplyRequest{Env: "dev", DryRun: true}))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if f.applyReq.Timeout != core.DefaultApplyTimeout {
		t.Errorf("timeout = %v, want default %v", f.applyReq.Timeout, core.DefaultApplyTimeout)
	}
	if !f.applyReq.DryRun {
		t.Error("dryRun not forwarded")
	}
}

func TestApplyTimeoutForwarded(t *testing.T) {
	f := &fakeUseCases{applyRes: &core.ApplyResult{}}
	cleaned := false
	_, err := srv(f, &cleaned).Apply(context.Background(),
		connect.NewRequest(&v1.ApplyRequest{Env: "dev", TimeoutSeconds: 30}))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if f.applyReq.Timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", f.applyReq.Timeout)
	}
}

func TestExecutePromotePlansThenExecutes(t *testing.T) {
	f := &fakeUseCases{
		promotePlan: &core.PromotePlan{From: "dev", To: "prod"},
		promoteRes:  &core.PromoteResult{From: "dev", To: "prod", CommitID: "1a2b3c"},
	}
	cleaned := false
	res, err := srv(f, &cleaned).ExecutePromote(context.Background(),
		connect.NewRequest(&v1.ExecutePromoteRequest{From: "dev", To: "prod"}))
	if err != nil {
		t.Fatalf("ExecutePromote: %v", err)
	}
	if f.planPromoteCalls != 1 || f.execPromoteCalls != 1 {
		t.Errorf("plan=%d exec=%d, want 1/1", f.planPromoteCalls, f.execPromoteCalls)
	}
	if res.Msg.GetCommitId() != "1a2b3c" {
		t.Errorf("commit = %q, want 1a2b3c", res.Msg.GetCommitId())
	}
}

func TestExecutePromotePlanErrorSkipsExecute(t *testing.T) {
	f := &fakeUseCases{promoteErr: &core.UnknownServiceError{Env: "prod", Service: "api"}}
	cleaned := false
	_, err := srv(f, &cleaned).ExecutePromote(context.Background(),
		connect.NewRequest(&v1.ExecutePromoteRequest{To: "prod", Service: "api"}))
	if err == nil {
		t.Fatal("expected error")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("code = %v, want InvalidArgument", connect.CodeOf(err))
	}
	if f.execPromoteCalls != 0 {
		t.Error("ExecutePromote called despite plan error")
	}
}

func TestHistoryConversion(t *testing.T) {
	ts := time.Date(2026, 7, 7, 10, 12, 0, 0, time.UTC)
	f := &fakeUseCases{historyRes: &core.HistoryResult{Entries: []store.Entry{{
		ID: 1, Time: ts, Actor: "arima", Action: store.ActionPromote,
		Env: "prod", Service: "api", Digest: "sha256:abc",
		Detail: map[string]string{"commit": "1a2b3c"},
	}}}}
	cleaned := false
	res, err := srv(f, &cleaned).History(context.Background(),
		connect.NewRequest(&v1.HistoryRequest{Env: "prod", Limit: 10}))
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(res.Msg.GetEntries()) != 1 {
		t.Fatalf("entries = %d, want 1", len(res.Msg.GetEntries()))
	}
	e := res.Msg.GetEntries()[0]
	if e.GetAction() != v1.Action_ACTION_PROMOTE {
		t.Errorf("action = %v, want PROMOTE", e.GetAction())
	}
	if !e.GetTime().AsTime().Equal(ts) {
		t.Errorf("time = %v, want %v", e.GetTime().AsTime(), ts)
	}
	if e.GetDetail()["commit"] != "1a2b3c" {
		t.Errorf("detail lost: %v", e.GetDetail())
	}
}

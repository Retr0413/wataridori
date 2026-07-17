// Command fakeserver serves the real Connect RPC handlers and the embedded
// web UI against canned data, so the frontend can be exercised without GCP
// credentials. Playwright starts it (see web/playwright.config.ts).
//
// The data is deliberately fixed — including timestamps — so E2E assertions
// and screenshots are reproducible. Because it implements server.UseCases,
// responses travel through the real converters and proto serialization: a
// change to the proto contract breaks these tests rather than sliding past
// hand-written fixtures.
//
// Mutations are not persisted; the fake is stateless and every request
// returns the same scenario.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/Retr0413/wataridori/internal/core"
	"github.com/Retr0413/wataridori/internal/server"
	"github.com/Retr0413/wataridori/internal/store"
	"github.com/Retr0413/wataridori/web"
)

// The scenario: checkout-api is fully promoted, web-frontend is one digest
// behind in prod (the promote case), and worker has drifted in dev and was
// never deployed to prod.
const (
	digestA = "sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	digestB = "sha256:9f8e7d6c5b4a9f8e7d6c5b4a9f8e7d6c5b4a9f8e7d6c5b4a9f8e7d6c5b4a9f8e"
	digestC = "sha256:3c2b1a0f9e8d3c2b1a0f9e8d3c2b1a0f9e8d3c2b1a0f9e8d3c2b1a0f9e8d3c2b"
	digestD = "sha256:5e4d3c2b1a095e4d3c2b1a095e4d3c2b1a095e4d3c2b1a095e4d3c2b1a095e4d"
)

const console = "https://console.cloud.google.com/run/detail/asia-northeast1"

// fixedTime anchors history so rendered timestamps never shift between runs.
var fixedTime = time.Date(2026, 7, 17, 9, 30, 0, 0, time.UTC)

type fake struct{}

func (fake) ListEnvironments(context.Context, core.ListEnvironmentsRequest) (*core.ListEnvironmentsResult, error) {
	return &core.ListEnvironmentsResult{Environments: []core.EnvironmentInfo{
		{Name: "dev", Policy: "auto", Branch: "develop", Project: "acme-dev", Region: "asia-northeast1"},
		{Name: "prod", Policy: "manual", PromoteFrom: "dev", Project: "acme-prod", Region: "asia-northeast1",
			ImageCopyTo: "asia-northeast1-docker.pkg.dev/acme-prod/images"},
	}}, nil
}

func (fake) Status(context.Context, core.StatusRequest) (*core.StatusResult, error) {
	return &core.StatusResult{Drift: true, Services: []core.ServiceStatus{
		{Env: "dev", Service: "checkout-api", DesiredDigest: digestA, ActualDigest: digestA,
			Revision: "checkout-api-00007", State: core.StateInSync, Ready: true, TrafficPct: 100,
			URL: "https://checkout-api-dev.a.run.app", ConsoleURL: console},
		{Env: "prod", Service: "checkout-api", DesiredDigest: digestA, ActualDigest: digestA,
			Revision: "checkout-api-00004", State: core.StateInSync, Ready: true, TrafficPct: 100,
			URL: "https://checkout-api-prod.a.run.app", ConsoleURL: console},
		{Env: "dev", Service: "web-frontend", DesiredDigest: digestB, ActualDigest: digestB,
			Revision: "web-frontend-00012", State: core.StateInSync, Ready: true, TrafficPct: 100,
			URL: "https://web-frontend-dev.a.run.app", ConsoleURL: console},
		{Env: "prod", Service: "web-frontend", DesiredDigest: digestC, ActualDigest: digestC,
			Revision: "web-frontend-00009", State: core.StateInSync, Ready: true, TrafficPct: 100,
			URL: "https://web-frontend-prod.a.run.app", ConsoleURL: console},
		{Env: "dev", Service: "worker", DesiredDigest: digestD, ActualDigest: digestC,
			Revision: "worker-00003", State: core.StateDrift, Ready: false,
			ReadyMessage: "Revision worker-00004 failed to become ready", TrafficPct: 100,
			ConsoleURL: console},
		{Env: "prod", Service: "worker", DesiredDigest: digestD, State: core.StateNotDeployed},
	}}, nil
}

func (fake) Inventory(_ context.Context, req core.InventoryRequest) (*core.InventoryResult, error) {
	all := []core.InventoryItem{
		{Env: "dev", Project: "acme-dev", Region: "asia-northeast1", Service: "checkout-api", Managed: true,
			DesiredDigest: digestA, ActualDigest: digestA, Revision: "checkout-api-00007", TrafficPct: 100,
			Ready: true, State: core.InventoryInSync, ConsoleURL: console},
		{Env: "dev", Project: "acme-dev", Region: "asia-northeast1", Service: "legacy-cron", Managed: false,
			ActualDigest: digestC, Revision: "legacy-cron-00002", TrafficPct: 100, Ready: true,
			State: core.InventoryUnmanaged, ConsoleURL: console},
		{Env: "prod", Project: "acme-prod", Region: "asia-northeast1", Service: "worker", Managed: true,
			DesiredDigest: digestD, State: core.InventoryNotDeployed},
	}
	if req.Env == "" {
		return &core.InventoryResult{Items: all}, nil
	}
	var out []core.InventoryItem
	for _, item := range all {
		if item.Env == req.Env {
			out = append(out, item)
		}
	}
	return &core.InventoryResult{Items: out}, nil
}

func (fake) Apply(_ context.Context, req core.ApplyRequest) (*core.ApplyResult, error) {
	return &core.ApplyResult{Env: req.Env, DryRun: req.DryRun}, nil
}

func (fake) PlanPromote(_ context.Context, req core.PromoteRequest) (*core.PromotePlan, error) {
	plan := &core.PromotePlan{From: req.From, To: req.To}
	// Only web-frontend is behind, so any other filter plans a no-op.
	if req.Service == "web-frontend" || req.Service == "" {
		plan.Items = append(plan.Items, core.PromoteItem{
			Service:   "web-frontend",
			FromImage: "asia-northeast1-docker.pkg.dev/acme-dev/images/web-frontend@" + digestB,
			OldImage:  "asia-northeast1-docker.pkg.dev/acme-prod/images/web-frontend@" + digestC,
			NewImage:  "asia-northeast1-docker.pkg.dev/acme-prod/images/web-frontend@" + digestB,
			NeedsCopy: true,
		})
	}
	return plan, nil
}

func (fake) ExecutePromote(_ context.Context, plan *core.PromotePlan) (*core.PromoteResult, error) {
	return &core.PromoteResult{From: plan.From, To: plan.To, CommitID: "abc1234def5678", Items: plan.Items}, nil
}

func (fake) PlanRollback(_ context.Context, req core.RollbackRequest) (*core.RollbackPlan, error) {
	return &core.RollbackPlan{Env: req.Env, Items: []core.RollbackItem{{
		Service:         req.Service,
		CurrentRevision: req.Service + "-00003",
		CurrentImage:    "reg.example/" + req.Service + "@" + digestC,
		TargetRevision:  req.Service + "-00002",
		TargetImage:     "reg.example/" + req.Service + "@" + digestA,
	}}}, nil
}

func (fake) ExecuteRollback(_ context.Context, plan *core.RollbackPlan) (*core.RollbackResult, error) {
	return &core.RollbackResult{Env: plan.Env, Items: plan.Items}, nil
}

func (fake) ListHistory(_ context.Context, req core.HistoryRequest) (*core.HistoryResult, error) {
	all := []store.Entry{
		{ID: 3, Time: fixedTime, Actor: "aoi@example.com", Action: store.ActionPromote,
			Env: "prod", Service: "checkout-api", Digest: digestA,
			Detail: map[string]string{"commit": "abc1234"}},
		{ID: 2, Time: fixedTime.Add(-3 * time.Hour), Actor: "ci@acme.iam.gserviceaccount.com",
			Action: store.ActionApply, Env: "dev", Service: "web-frontend", Digest: digestB,
			Detail: map[string]string{"revision": "web-frontend-00012"}},
		{ID: 1, Time: fixedTime.Add(-26 * time.Hour), Actor: "aoi@example.com",
			Action: store.ActionRollback, Env: "prod", Service: "worker", Digest: digestC,
			Detail: map[string]string{"revision": "worker-00003"}},
	}
	if req.Env == "" {
		return &core.HistoryResult{Entries: all}, nil
	}
	var out []store.Entry
	for _, e := range all {
		if e.Env == req.Env {
			out = append(out, e)
		}
	}
	return &core.HistoryResult{Entries: out}, nil
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8137", "address to listen on")
	flag.Parse()

	srv := server.New(func(context.Context) (server.UseCases, func(), error) {
		return fake{}, func() {}, nil
	})
	path, handler := srv.Handler()
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.Handle("/", web.Handler())

	fmt.Println("fakeserver listening on", *addr)
	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := httpSrv.ListenAndServe(); err != nil {
		panic(err)
	}
}

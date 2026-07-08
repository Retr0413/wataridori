package controller

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Retr0413/wataridori/internal/core"
)

// fakeDeployer records applied environments and can fail selected ones.
type fakeDeployer struct {
	mu      sync.Mutex
	applied []string
	failEnv string
}

func (d *fakeDeployer) Apply(_ context.Context, req core.ApplyRequest) (*core.ApplyResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if req.Env == d.failEnv {
		return nil, errors.New("boom")
	}
	d.applied = append(d.applied, req.Env)
	return &core.ApplyResult{Env: req.Env}, nil
}

func (d *fakeDeployer) snapshot() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.applied...)
}

func TestReconcileAppliesAutoEnvs(t *testing.T) {
	dep := &fakeDeployer{}
	c := New(0, func(context.Context) (Snapshot, func(), error) {
		return Snapshot{AutoEnvs: []string{"dev", "staging"}, Deployer: dep}, func() {}, nil
	})
	c.reconcile(context.Background())

	got := dep.snapshot()
	if len(got) != 2 || got[0] != "dev" || got[1] != "staging" {
		t.Fatalf("applied = %v, want [dev staging]", got)
	}
}

func TestReconcileContinuesPastFailingEnv(t *testing.T) {
	dep := &fakeDeployer{failEnv: "dev"}
	var logs []string
	c := New(0, func(context.Context) (Snapshot, func(), error) {
		return Snapshot{AutoEnvs: []string{"dev", "prod"}, Deployer: dep}, func() {}, nil
	})
	c.Logf = func(format string, args ...any) { logs = append(logs, format) }
	c.reconcile(context.Background())

	if got := dep.snapshot(); len(got) != 1 || got[0] != "prod" {
		t.Fatalf("applied = %v, want [prod] (dev failed but loop continued)", got)
	}
}

func TestReconcileSkipsCycleOnRefreshError(t *testing.T) {
	dep := &fakeDeployer{}
	c := New(0, func(context.Context) (Snapshot, func(), error) {
		return Snapshot{AutoEnvs: []string{"dev"}, Deployer: dep}, func() {}, nil
	})
	c.Refresh = func(context.Context) error { return errors.New("git pull failed") }
	c.reconcile(context.Background())

	if got := dep.snapshot(); len(got) != 0 {
		t.Fatalf("applied = %v, want none (refresh failed)", got)
	}
}

func TestReconcileCleanupAlwaysCalled(t *testing.T) {
	cleaned := false
	c := New(0, func(context.Context) (Snapshot, func(), error) {
		return Snapshot{}, func() { cleaned = true }, errors.New("load failed")
	})
	c.reconcile(context.Background())
	if !cleaned {
		t.Error("cleanup not called on load error")
	}
}

func TestRunReconcilesAtStartupThenStops(t *testing.T) {
	dep := &fakeDeployer{}
	c := New(0, func(context.Context) (Snapshot, func(), error) {
		return Snapshot{AutoEnvs: []string{"dev"}, Deployer: dep}, func() {}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	// The startup reconcile should apply dev without waiting for a tick.
	if !waitFor(func() bool { return len(dep.snapshot()) == 1 }, time.Second) {
		t.Fatal("startup reconcile did not run")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("Run err = %v, want context.Canceled", err)
	}
}

func TestTriggerCausesReconcile(t *testing.T) {
	dep := &fakeDeployer{}
	c := New(0, func(context.Context) (Snapshot, func(), error) {
		return Snapshot{AutoEnvs: []string{"dev"}, Deployer: dep}, func() {}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run(ctx) }()

	// Wait for the startup cycle, then trigger a second one.
	if !waitFor(func() bool { return len(dep.snapshot()) == 1 }, time.Second) {
		t.Fatal("startup reconcile did not run")
	}
	c.Trigger()
	if !waitFor(func() bool { return len(dep.snapshot()) == 2 }, time.Second) {
		t.Fatal("trigger did not cause a reconcile")
	}
}

func TestRunWithoutLoad(t *testing.T) {
	c := &Controller{}
	if err := c.Run(context.Background()); !errors.Is(err, ErrNoLoad) {
		t.Errorf("Run err = %v, want ErrNoLoad", err)
	}
}

func waitFor(cond func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return cond()
}

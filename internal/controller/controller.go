package controller

import (
	"context"
	"errors"
	"time"

	"github.com/Retr0413/wataridori/internal/core"
)

// Deployer applies an environment's desired state. *core.Engine satisfies it.
type Deployer interface {
	Apply(context.Context, core.ApplyRequest) (*core.ApplyResult, error)
}

// Snapshot is one reconcile target set, rebuilt each cycle so the loop tracks
// the current manifest state. Deployer is bound to the same freshly loaded
// repository as AutoEnvs.
type Snapshot struct {
	AutoEnvs []string
	Deployer Deployer
}

// LoadFunc reloads the manifest repo and returns the auto environments plus a
// Deployer. The cleanup is always called when the cycle finishes.
type LoadFunc func(context.Context) (Snapshot, func(), error)

// Controller reconciles policy:auto environments on an interval or on demand.
type Controller struct {
	// Interval between periodic reconciles. Values <= 0 disable the ticker;
	// the loop then only reconciles on trigger.
	Interval time.Duration
	// Load rebuilds the snapshot each cycle.
	Load LoadFunc
	// Refresh, if set, runs before Load each cycle (e.g. git pull). A refresh
	// error is logged and skips the cycle rather than stopping the loop.
	Refresh func(context.Context) error
	// Logf reports progress and per-environment errors. Defaults to no-op.
	Logf func(string, ...any)

	trigger chan struct{}
}

// New returns a ready Controller. Interval <= 0 means trigger-only.
func New(interval time.Duration, load LoadFunc) *Controller {
	return &Controller{
		Interval: interval,
		Load:     load,
		// Buffered so Trigger never blocks; one pending kick is enough.
		trigger: make(chan struct{}, 1),
	}
}

// Trigger requests a reconcile as soon as possible (e.g. from a webhook).
// It never blocks; a kick already pending is coalesced.
func (c *Controller) Trigger() {
	if c.trigger == nil {
		return
	}
	select {
	case c.trigger <- struct{}{}:
	default:
	}
}

func (c *Controller) logf(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
	}
}

// Run reconciles once immediately, then on every interval tick and trigger,
// until ctx is cancelled. It returns ctx.Err() (nil on a clean stop is not
// possible; cancellation is the only exit).
func (c *Controller) Run(ctx context.Context) error {
	if c.Load == nil {
		return ErrNoLoad
	}
	if c.trigger == nil {
		c.trigger = make(chan struct{}, 1)
	}

	var tick <-chan time.Time
	if c.Interval > 0 {
		t := time.NewTicker(c.Interval)
		defer t.Stop()
		tick = t.C
	}

	// Reconcile once at startup so auto envs converge without waiting a tick.
	c.reconcile(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick:
			c.reconcile(ctx)
		case <-c.trigger:
			c.reconcile(ctx)
		}
	}
}

// reconcile runs one cycle. It never returns an error: a failure in one
// environment must not stop the loop or skip the others.
func (c *Controller) reconcile(ctx context.Context) {
	if c.Refresh != nil {
		if err := c.Refresh(ctx); err != nil {
			c.logf("reconcile: refresh failed, skipping cycle: %v", err)
			return
		}
	}

	snap, cleanup, err := c.Load(ctx)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		c.logf("reconcile: load failed: %v", err)
		return
	}
	if len(snap.AutoEnvs) == 0 {
		return
	}

	for _, env := range snap.AutoEnvs {
		if ctx.Err() != nil {
			return
		}
		res, err := snap.Deployer.Apply(ctx, core.ApplyRequest{
			Env:     env,
			Timeout: core.DefaultApplyTimeout,
		})
		if err != nil {
			c.logf("reconcile: apply %s failed: %v", env, err)
			continue
		}
		c.logf("reconcile: %s reconciled (%d services)", env, len(res.Services))
	}
}

// ErrNoLoad is returned by Run when Load is nil.
var ErrNoLoad = errors.New("controller: Load is nil")

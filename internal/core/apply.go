package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Retr0413/wataridori/internal/manifest"
	"github.com/Retr0413/wataridori/internal/store"
)

// ApplyRequest deploys the manifests of one environment.
// See docs/spec/phase1-cli.md §2.1.
type ApplyRequest struct {
	Env     string `json:"env"`
	Service string `json:"service,omitempty"` // optional filter
	DryRun  bool   `json:"dryRun,omitempty"`
	// Force applies even when the running service has configuration the
	// manifest cannot express. Without it such an apply is refused, because
	// the replacement would silently drop that configuration.
	Force   bool          `json:"force,omitempty"`
	Timeout time.Duration `json:"-"`
}

// UnmanagedSettingsError refuses an apply that would drop configuration the
// manifest does not describe. Cloud Run services created by Terraform or
// gcloud routinely carry probes, volumes or a custom request timeout that
// Wataridori's small schema has no field for.
type UnmanagedSettingsError struct {
	Env      string
	Service  string
	Settings []string
}

func (e *UnmanagedSettingsError) Error() string {
	return fmt.Sprintf(
		"service %q in %q has configuration the manifest cannot express, and apply replaces the service wholesale: %s would be removed. "+
			"Move the settings into Terraform-managed infrastructure, or pass --force to apply anyway.",
		e.Service, e.Env, strings.Join(e.Settings, ", "))
}

// ApplyResult is structured data; rendering belongs to the caller.
type ApplyResult struct {
	Env      string               `json:"env"`
	DryRun   bool                 `json:"dryRun,omitempty"`
	Services []ApplyServiceResult `json:"services"`
}

type ApplyServiceResult struct {
	Service string `json:"service"`
	// RunName is the Cloud Run service actually written.
	RunName      string `json:"runName,omitempty"`
	DesiredImage string `json:"desiredImage"`
	// ActualImage is what served traffic before this apply ("" = new service).
	ActualImage string `json:"actualImage,omitempty"`
	// Revision is the revision serving after the apply (empty on dry-run).
	Revision string `json:"revision,omitempty"`
	URL      string `json:"url,omitempty"`
	// InSync means desired == actual; a dry-run reports it without deploying.
	InSync bool `json:"inSync"`
	// Unmanaged lists settings on the running service that this apply would
	// drop. Populated on a dry-run, and on a forced apply.
	Unmanaged []string `json:"unmanaged,omitempty"`
}

func (e *Engine) Apply(ctx context.Context, req ApplyRequest) (*ApplyResult, error) {
	env, err := e.Repo.Environment(req.Env)
	if err != nil {
		return nil, err
	}
	services, err := e.services(env, req.Service)
	if err != nil {
		return nil, err
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = DefaultApplyTimeout
	}

	result := &ApplyResult{Env: env.Name, DryRun: req.DryRun}
	for _, svc := range services {
		actual, err := e.CloudRun.Get(ctx, env, svc.RunName())
		if err != nil {
			return nil, err
		}
		item := ApplyServiceResult{Service: svc.Name, RunName: svc.RunName(), DesiredImage: svc.Image}
		if actual != nil {
			item.ActualImage = actual.Image
			item.InSync = actual.Image == svc.Image
		}

		// Checked before deploying, not after: the point is to stop an apply
		// that would quietly strip settings off a running service.
		unmanaged, err := e.CloudRun.UnmanagedSettings(ctx, env, svc)
		if err != nil {
			return nil, err
		}
		if len(unmanaged) > 0 {
			if !req.DryRun && !req.Force {
				return nil, &UnmanagedSettingsError{Env: env.Name, Service: svc.Name, Settings: unmanaged}
			}
			item.Unmanaged = unmanaged
		}

		if !req.DryRun {
			deployed, err := e.CloudRun.Apply(ctx, env, svc, timeout)
			if err != nil {
				return nil, err
			}
			item.Revision = deployed.Revision
			item.URL = deployed.URL
			item.InSync = true

			_, digest, err := manifest.SplitDigest(svc.Image)
			if err != nil {
				return nil, err
			}
			detail := map[string]string{"revision": deployed.Revision}
			if err := e.record(ctx, store.ActionApply, env.Name, svc.Name, digest, detail); err != nil {
				return nil, err
			}
		}
		result.Services = append(result.Services, item)
	}
	return result, nil
}

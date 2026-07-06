package core

import (
	"context"
	"time"

	"github.com/Retr0413/wataridori/internal/manifest"
	"github.com/Retr0413/wataridori/internal/store"
)

// ApplyRequest deploys the manifests of one environment.
// See docs/spec/phase1-cli.md §2.1.
type ApplyRequest struct {
	Env     string        `json:"env"`
	Service string        `json:"service,omitempty"` // optional filter
	DryRun  bool          `json:"dryRun,omitempty"`
	Timeout time.Duration `json:"-"`
}

// ApplyResult is structured data; rendering belongs to the caller.
type ApplyResult struct {
	Env      string               `json:"env"`
	DryRun   bool                 `json:"dryRun,omitempty"`
	Services []ApplyServiceResult `json:"services"`
}

type ApplyServiceResult struct {
	Service      string `json:"service"`
	DesiredImage string `json:"desiredImage"`
	// ActualImage is what served traffic before this apply ("" = new service).
	ActualImage string `json:"actualImage,omitempty"`
	// Revision is the revision serving after the apply (empty on dry-run).
	Revision string `json:"revision,omitempty"`
	URL      string `json:"url,omitempty"`
	// InSync means desired == actual; a dry-run reports it without deploying.
	InSync bool `json:"inSync"`
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
		actual, err := e.CloudRun.Get(ctx, env, svc.Name)
		if err != nil {
			return nil, err
		}
		item := ApplyServiceResult{Service: svc.Name, DesiredImage: svc.Image}
		if actual != nil {
			item.ActualImage = actual.Image
			item.InSync = actual.Image == svc.Image
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

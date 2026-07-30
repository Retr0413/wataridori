package core

import (
	"context"
	"fmt"

	"github.com/Retr0413/wataridori/internal/cloudrun"
	"github.com/Retr0413/wataridori/internal/manifest"
	"github.com/Retr0413/wataridori/internal/store"
)

// RollbackRequest switches traffic back to a previous revision.
// See docs/spec/phase1-cli.md §2.3.
type RollbackRequest struct {
	Env     string `json:"env"`
	Service string `json:"service,omitempty"` // optional filter
	// Revision pins the rollback target explicitly (single service only).
	Revision string `json:"revision,omitempty"`
}

// RollbackPlan is the preview shown before the confirm prompt.
type RollbackPlan struct {
	Env   string         `json:"env"`
	Items []RollbackItem `json:"items"`

	env *manifest.Environment
}

type RollbackItem struct {
	Service string `json:"service"`
	// RunName is the Cloud Run service whose traffic moves.
	RunName         string `json:"runName,omitempty"`
	CurrentRevision string `json:"currentRevision"`
	CurrentImage    string `json:"currentImage"`
	TargetRevision  string `json:"targetRevision"`
	TargetImage     string `json:"targetImage"`
}

// RollbackResult reports the switch. The manifest is intentionally not
// touched: status will show the drift until the manifest catches up.
type RollbackResult struct {
	Env   string         `json:"env"`
	Items []RollbackItem `json:"items"`
}

func (e *Engine) PlanRollback(ctx context.Context, req RollbackRequest) (*RollbackPlan, error) {
	env, err := e.Repo.Environment(req.Env)
	if err != nil {
		return nil, err
	}
	services, err := e.services(env, req.Service)
	if err != nil {
		return nil, err
	}
	if req.Revision != "" && len(services) != 1 {
		return nil, fmt.Errorf("--revision requires --service (environment %q has %d services)", env.Name, len(services))
	}

	plan := &RollbackPlan{Env: env.Name, env: env}
	for _, svc := range services {
		revisions, err := e.CloudRun.ListRevisions(ctx, env, svc.RunName())
		if err != nil {
			return nil, err
		}
		item, err := rollbackTarget(svc.Name, revisions, req.Revision)
		if err != nil {
			return nil, err
		}
		item.RunName = svc.RunName()
		plan.Items = append(plan.Items, *item)
	}
	return plan, nil
}

// rollbackTarget picks the newest ready revision older than the one
// currently serving traffic (revisions are newest first).
func rollbackTarget(service string, revisions []cloudrun.Revision, explicit string) (*RollbackItem, error) {
	current := -1
	for i, rev := range revisions {
		if rev.TrafficPercent > 0 {
			current = i
			break
		}
	}
	if current < 0 {
		return nil, fmt.Errorf("service %q: no revision is serving traffic", service)
	}
	item := &RollbackItem{
		Service:         service,
		CurrentRevision: revisions[current].Name,
		CurrentImage:    revisions[current].Image,
	}

	if explicit != "" {
		for _, rev := range revisions {
			if rev.Name != explicit {
				continue
			}
			if !rev.Ready {
				return nil, fmt.Errorf("revision %q is not ready", explicit)
			}
			item.TargetRevision = rev.Name
			item.TargetImage = rev.Image
			return item, nil
		}
		return nil, fmt.Errorf("service %q has no revision named %q", service, explicit)
	}

	for _, rev := range revisions[current+1:] {
		if rev.Ready {
			item.TargetRevision = rev.Name
			item.TargetImage = rev.Image
			return item, nil
		}
	}
	return nil, fmt.Errorf("service %q: no ready revision older than %s to roll back to", service, item.CurrentRevision)
}

func (e *Engine) ExecuteRollback(ctx context.Context, plan *RollbackPlan) (*RollbackResult, error) {
	for _, item := range plan.Items {
		if err := e.CloudRun.SetTraffic(ctx, plan.env, item.RunName, item.TargetRevision); err != nil {
			return nil, err
		}
		digest := ""
		if _, d, err := manifest.SplitDigest(item.TargetImage); err == nil {
			digest = d
		}
		detail := map[string]string{"revision": item.TargetRevision, "rolledBackFrom": item.CurrentRevision}
		if err := e.record(ctx, store.ActionRollback, plan.Env, item.Service, digest, detail); err != nil {
			return nil, err
		}
	}
	return &RollbackResult{Env: plan.Env, Items: plan.Items}, nil
}

package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Retr0413/wataridori/internal/gitops"
	"github.com/Retr0413/wataridori/internal/manifest"
	"github.com/Retr0413/wataridori/internal/store"
)

// PromoteRequest copies digests from one environment's manifests to
// another's. See docs/spec/phase1-cli.md §2.2.
type PromoteRequest struct {
	// From is optional; empty means the target's promoteFrom.
	From    string `json:"from,omitempty"`
	To      string `json:"to"`
	Service string `json:"service,omitempty"` // optional filter
}

// PromotePlan is the preview shown before the confirm prompt. Only items
// with a digest difference are included.
type PromotePlan struct {
	From  string        `json:"from"`
	To    string        `json:"to"`
	Items []PromoteItem `json:"items"`

	fromEnv *manifest.Environment
	toEnv   *manifest.Environment
}

type PromoteItem struct {
	Service string `json:"service"`
	// FromImage is the digest-pinned source reference (copy source).
	FromImage string `json:"fromImage"`
	// OldImage / NewImage are the target manifest's image before and after.
	OldImage string `json:"oldImage"`
	NewImage string `json:"newImage"`
	// NeedsCopy is set when the target environment configures imageCopy.
	NeedsCopy bool `json:"needsCopy"`

	svc *manifest.Service
}

// PromoteResult reports what was written. Promote does not deploy; that is
// apply's job (GitOps separation).
type PromoteResult struct {
	From     string        `json:"from"`
	To       string        `json:"to"`
	CommitID string        `json:"commitId"`
	Items    []PromoteItem `json:"items"`
}

// PlanPromote validates the request and computes the digest diff.
// An empty Items slice means everything is already promoted (no-op).
func (e *Engine) PlanPromote(_ context.Context, req PromoteRequest) (*PromotePlan, error) {
	toEnv, err := e.Repo.Environment(req.To)
	if err != nil {
		return nil, err
	}
	if toEnv.Policy == manifest.PolicyAuto {
		return nil, fmt.Errorf("environment %q has policy \"auto\" (branch %s); promoting into an auto environment is contradictory", toEnv.Name, toEnv.Branch)
	}

	fromName := req.From
	if fromName == "" {
		fromName = toEnv.PromoteFrom
	}
	if fromName == "" {
		return nil, fmt.Errorf("environment %q has no promoteFrom; pass --from explicitly", toEnv.Name)
	}
	if fromName == toEnv.Name {
		return nil, fmt.Errorf("cannot promote %q into itself", fromName)
	}
	fromEnv, err := e.Repo.Environment(fromName)
	if err != nil {
		return nil, err
	}

	fromServices, err := e.Repo.LoadServices(fromEnv)
	if err != nil {
		return nil, err
	}
	fromByName := make(map[string]*manifest.Service, len(fromServices))
	for _, svc := range fromServices {
		fromByName[svc.Name] = svc
	}

	toServices, err := e.services(toEnv, req.Service)
	if err != nil {
		return nil, err
	}

	plan := &PromotePlan{From: fromEnv.Name, To: toEnv.Name, fromEnv: fromEnv, toEnv: toEnv}
	for _, toSvc := range toServices {
		fromSvc, ok := fromByName[toSvc.Name]
		if !ok {
			return nil, fmt.Errorf("service %q exists in %q but not in %q", toSvc.Name, toEnv.Name, fromEnv.Name)
		}
		_, fromDigest, err := manifest.SplitDigest(fromSvc.Image)
		if err != nil {
			return nil, err
		}
		toPath, toDigest, err := manifest.SplitDigest(toSvc.Image)
		if err != nil {
			return nil, err
		}
		if fromDigest == toDigest {
			continue
		}
		// Only the digest moves; the image path stays the target's (spec §1.3).
		item := PromoteItem{
			Service:   toSvc.Name,
			FromImage: fromSvc.Image,
			OldImage:  toSvc.Image,
			NewImage:  manifest.WithDigest(toPath, fromDigest),
			NeedsCopy: toEnv.ImageCopy != nil,
			svc:       toSvc,
		}
		if item.NeedsCopy && !strings.HasPrefix(toPath, toEnv.ImageCopy.To+"/") && toPath != toEnv.ImageCopy.To {
			return nil, fmt.Errorf("service %q: image path %s is not under imageCopy.to %s", toSvc.Name, toPath, toEnv.ImageCopy.To)
		}
		plan.Items = append(plan.Items, item)
	}
	return plan, nil
}

// ExecutePromote copies images if configured, rewrites the target
// manifests, and records everything as one git commit.
func (e *Engine) ExecutePromote(ctx context.Context, plan *PromotePlan) (*PromoteResult, error) {
	if len(plan.Items) == 0 {
		return &PromoteResult{From: plan.From, To: plan.To, Items: nil}, nil
	}

	var files []string
	for i := range plan.Items {
		item := &plan.Items[i]
		if item.NeedsCopy {
			toPath, _, err := manifest.SplitDigest(item.NewImage)
			if err != nil {
				return nil, err
			}
			if _, err := e.Copier.Copy(ctx, item.FromImage, toPath); err != nil {
				return nil, err
			}
		}
		if err := e.Repo.UpdateServiceImage(item.svc, item.NewImage); err != nil {
			return nil, err
		}
		files = append(files, filepath.Join(e.Repo.Root, item.svc.File))
	}

	message := promoteMessage(plan)
	commitID, err := e.Commit.Commit(e.Repo.Root, files, message)
	if err != nil {
		return nil, err
	}

	for _, item := range plan.Items {
		_, digest, err := manifest.SplitDigest(item.NewImage)
		if err != nil {
			return nil, err
		}
		detail := map[string]string{"from": plan.From, "commit": commitID}
		if err := e.record(ctx, store.ActionPromote, plan.To, item.Service, digest, detail); err != nil {
			return nil, err
		}
	}
	return &PromoteResult{From: plan.From, To: plan.To, CommitID: commitID, Items: plan.Items}, nil
}

func promoteMessage(plan *PromotePlan) string {
	if len(plan.Items) == 1 {
		item := plan.Items[0]
		_, digest, _ := manifest.SplitDigest(item.NewImage)
		return gitops.PromoteMessage(plan.To, item.Service, manifest.ShortDigest(digest), plan.From)
	}
	names := make([]string, len(plan.Items))
	for i, item := range plan.Items {
		names[i] = item.Service
	}
	return fmt.Sprintf("promote(%s): %s (from %s)", plan.To, strings.Join(names, ", "), plan.From)
}

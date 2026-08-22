package core

import (
	"fmt"
	"sort"

	"github.com/Retr0413/wataridori/internal/manifest"
)

// ValidateResult is the provider-neutral summary returned by the offline
// manifest validation command.
type ValidateResult struct {
	Root         string   `json:"root"`
	Environments int      `json:"environments"`
	Services     int      `json:"services"`
	Warnings     []string `json:"warnings,omitempty"`
}

// ValidateManifests loads every service manifest and checks promotion
// relationships without contacting Git, a registry, Cloud Run, or history.
func (e *Engine) ValidateManifests() (*ValidateResult, error) {
	names := make([]string, 0, len(e.Repo.Config.Environments))
	for name := range e.Repo.Config.Environments {
		names = append(names, name)
	}
	sort.Strings(names)

	byEnv := make(map[string]map[string]struct{}, len(names))
	result := &ValidateResult{
		Root:         e.Repo.Root,
		Environments: len(names),
		Warnings:     append([]string(nil), e.Repo.Warnings...),
	}
	for _, name := range names {
		env := e.Repo.Config.Environments[name]
		services, err := e.Repo.LoadServices(env)
		if err != nil {
			return nil, err
		}
		result.Services += len(services)
		set := make(map[string]struct{}, len(services))
		for _, svc := range services {
			set[svc.Name] = struct{}{}
		}
		byEnv[name] = set
	}

	for _, name := range names {
		env := e.Repo.Config.Environments[name]
		if env.PromoteFrom == "" {
			continue
		}
		for service := range byEnv[name] {
			if _, ok := byEnv[env.PromoteFrom][service]; !ok {
				return nil, fmt.Errorf("environment %q: service %q is missing from promoteFrom environment %q", name, service, env.PromoteFrom)
			}
		}
	}
	return result, nil
}

// SetManifestImageRequest selects one manifest and its externally built,
// digest-pinned image. It changes Git desired state only; committing and
// opening a pull request belong to the caller.
type SetManifestImageRequest struct {
	Env           string          `json:"env"`
	Service       string          `json:"service"`
	Image         string          `json:"image"`
	RequirePolicy manifest.Policy `json:"requirePolicy,omitempty"`
}

type SetManifestImageResult struct {
	Env      string `json:"env"`
	Service  string `json:"service"`
	File     string `json:"file"`
	OldImage string `json:"oldImage"`
	NewImage string `json:"newImage"`
	Changed  bool   `json:"changed"`
}

// SetManifestImage writes one service image while preserving the manifest's
// formatting and comments.
func (e *Engine) SetManifestImage(req SetManifestImageRequest) (*SetManifestImageResult, error) {
	if req.Service == "" {
		return nil, fmt.Errorf("service is required")
	}
	if _, _, err := manifest.SplitDigest(req.Image); err != nil {
		return nil, err
	}
	env, err := e.Repo.Environment(req.Env)
	if err != nil {
		return nil, err
	}
	if req.RequirePolicy != "" && env.Policy != req.RequirePolicy {
		return nil, fmt.Errorf("environment %q uses policy %q; policy %q is required", env.Name, env.Policy, req.RequirePolicy)
	}
	services, err := e.services(env, req.Service)
	if err != nil {
		return nil, err
	}
	svc := services[0]
	result := &SetManifestImageResult{
		Env:      env.Name,
		Service:  svc.Name,
		File:     svc.File,
		OldImage: svc.Image,
		NewImage: req.Image,
		Changed:  svc.Image != req.Image,
	}
	if !result.Changed {
		return result, nil
	}
	if err := e.Repo.UpdateServiceImage(svc, req.Image); err != nil {
		return nil, err
	}
	return result, nil
}

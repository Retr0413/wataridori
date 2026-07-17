package core

import (
	"context"
	"sort"

	"github.com/Retr0413/wataridori/internal/manifest"
)

// ListEnvironmentsRequest takes no arguments; the whole configuration is
// returned. It exists so the use case matches the shape of the others.
type ListEnvironmentsRequest struct{}

// EnvironmentInfo describes one configured environment. Everything here comes
// from wataridori.yaml, so listing environments costs no Cloud Run calls.
type EnvironmentInfo struct {
	Name        string `json:"name"`
	Policy      string `json:"policy"`
	Branch      string `json:"branch,omitempty"`
	PromoteFrom string `json:"promoteFrom,omitempty"`
	Project     string `json:"project"`
	Region      string `json:"region"`
	ImageCopyTo string `json:"imageCopyTo,omitempty"`
}

// ListEnvironmentsResult lists environments in promotion order.
type ListEnvironmentsResult struct {
	Environments []EnvironmentInfo `json:"environments"`
}

// ListEnvironments returns the configured environments ordered so that a
// promotion source precedes its targets. The Web UI lays out one column per
// environment and needs that order before any status arrives.
func (e *Engine) ListEnvironments(_ context.Context, _ ListEnvironmentsRequest) (*ListEnvironmentsResult, error) {
	envs := orderEnvironments(e.Repo.Config.Environments)
	result := &ListEnvironmentsResult{Environments: make([]EnvironmentInfo, 0, len(envs))}
	for _, env := range envs {
		info := EnvironmentInfo{
			Name:        env.Name,
			Policy:      string(env.Policy),
			Branch:      env.Branch,
			PromoteFrom: env.PromoteFrom,
			Project:     env.GCP.Project,
			Region:      env.GCP.Region,
		}
		if env.ImageCopy != nil {
			info.ImageCopyTo = env.ImageCopy.To
		}
		result.Environments = append(result.Environments, info)
	}
	return result, nil
}

// orderEnvironments topologically sorts environments along promoteFrom edges,
// breaking ties by name so the order is stable across calls.
//
// The manifest validator only checks promoteFrom for policy:manual, and it
// does not reject cycles, so neither can be assumed here: unresolvable edges
// are ignored, and environments left in a cycle are appended by name. The
// listing stays total — every configured environment comes back exactly once.
func orderEnvironments(envs map[string]*manifest.Environment) []*manifest.Environment {
	// pending counts the unplaced sources each environment waits on, and
	// targets maps an environment to those promoting from it.
	pending := make(map[string]int, len(envs))
	targets := make(map[string][]string, len(envs))
	for name, env := range envs {
		src := env.PromoteFrom
		if src == "" || src == name {
			continue
		}
		if _, ok := envs[src]; !ok {
			continue
		}
		pending[name]++
		targets[src] = append(targets[src], name)
	}

	ready := make([]string, 0, len(envs))
	for name := range envs {
		if pending[name] == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)

	ordered := make([]*manifest.Environment, 0, len(envs))
	placed := make(map[string]bool, len(envs))
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		ordered = append(ordered, envs[name])
		placed[name] = true
		for _, target := range targets[name] {
			pending[target]--
			if pending[target] == 0 {
				ready = append(ready, target)
			}
		}
		sort.Strings(ready)
	}

	// Environments in a promoteFrom cycle never reach zero pending sources.
	if len(ordered) == len(envs) {
		return ordered
	}
	cycled := make([]string, 0, len(envs)-len(ordered))
	for name := range envs {
		if !placed[name] {
			cycled = append(cycled, name)
		}
	}
	sort.Strings(cycled)
	for _, name := range cycled {
		ordered = append(ordered, envs[name])
	}
	return ordered
}

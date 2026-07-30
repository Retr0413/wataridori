package core

import (
	"context"
	"sort"

	"github.com/Retr0413/wataridori/internal/manifest"
)

// StatusRequest compares desired state (Git) with observed state (Cloud
// Run). See docs/spec/phase1-cli.md §2.4.
type StatusRequest struct {
	Env string `json:"env,omitempty"` // empty = all environments
}

// SyncState classifies one service's desired-vs-actual comparison.
type SyncState string

const (
	StateInSync      SyncState = "in sync"
	StateDrift       SyncState = "drift"
	StateNotDeployed SyncState = "not deployed"
)

// ServiceStatus carries everything the CLI table shows today plus the
// fields the Phase 2 UI needs (traffic, ready reason, deep links).
type ServiceStatus struct {
	Env string `json:"env"`
	// Service is the manifest identity, shared across environments.
	Service string `json:"service"`
	// RunName is the Cloud Run service behind it, which may differ per
	// environment (manifest.Service.CloudRunName).
	RunName       string    `json:"runName,omitempty"`
	DesiredImage  string    `json:"desiredImage"`
	DesiredDigest string    `json:"desiredDigest"`
	ActualImage   string    `json:"actualImage,omitempty"`
	ActualDigest  string    `json:"actualDigest,omitempty"`
	Revision      string    `json:"revision,omitempty"`
	State         SyncState `json:"state"`
	Ready         bool      `json:"ready"`
	ReadyMessage  string    `json:"readyMessage,omitempty"`
	TrafficPct    int32     `json:"trafficPercent"`
	URL           string    `json:"url,omitempty"`
	ConsoleURL    string    `json:"consoleUrl,omitempty"`
}

type StatusResult struct {
	Services []ServiceStatus `json:"services"`
	// Drift is true when any service is not in sync (exit code 2 with --check).
	Drift bool `json:"drift"`
}

func (e *Engine) Status(ctx context.Context, req StatusRequest) (*StatusResult, error) {
	var envs []*manifest.Environment
	if req.Env != "" {
		env, err := e.Repo.Environment(req.Env)
		if err != nil {
			return nil, err
		}
		envs = []*manifest.Environment{env}
	} else {
		for _, env := range e.Repo.Config.Environments {
			envs = append(envs, env)
		}
		sort.Slice(envs, func(i, j int) bool { return envs[i].Name < envs[j].Name })
	}

	result := &StatusResult{}
	for _, env := range envs {
		services, err := e.Repo.LoadServices(env)
		if err != nil {
			return nil, err
		}
		for _, svc := range services {
			_, desiredDigest, err := manifest.SplitDigest(svc.Image)
			if err != nil {
				return nil, err
			}
			st := ServiceStatus{
				Env:           env.Name,
				Service:       svc.Name,
				RunName:       svc.RunName(),
				DesiredImage:  svc.Image,
				DesiredDigest: desiredDigest,
			}

			actual, err := e.CloudRun.Get(ctx, env, svc.RunName())
			if err != nil {
				return nil, err
			}
			if actual == nil {
				st.State = StateNotDeployed
			} else {
				st.ActualImage = actual.Image
				st.Revision = actual.Revision
				st.Ready = actual.Ready
				st.ReadyMessage = actual.ReadyMessage
				st.TrafficPct = actual.TrafficPercent
				st.URL = actual.URL
				st.ConsoleURL = actual.ConsoleURL
				if _, d, err := manifest.SplitDigest(actual.Image); err == nil {
					st.ActualDigest = d
				}
				if st.ActualDigest == desiredDigest {
					st.State = StateInSync
				} else {
					st.State = StateDrift
				}
			}
			if st.State != StateInSync {
				result.Drift = true
			}
			result.Services = append(result.Services, st)
		}
	}
	return result, nil
}

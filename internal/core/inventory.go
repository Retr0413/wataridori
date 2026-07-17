package core

import (
	"context"
	"sort"

	"github.com/Retr0413/wataridori/internal/cloudrun"
	"github.com/Retr0413/wataridori/internal/manifest"
)

// InventoryRequest lists Cloud Run services and marks whether Wataridori
// manages them through the manifest repository.
type InventoryRequest struct {
	Env string `json:"env,omitempty"` // empty = all environments
}

type InventoryState string

const (
	InventoryInSync      InventoryState = "in sync"
	InventoryDrift       InventoryState = "drift"
	InventoryNotDeployed InventoryState = "not deployed"
	InventoryUnmanaged   InventoryState = "unmanaged"
)

type InventoryItem struct {
	Env           string         `json:"env"`
	Project       string         `json:"project"`
	Region        string         `json:"region"`
	Service       string         `json:"service"`
	Managed       bool           `json:"managed"`
	DesiredImage  string         `json:"desiredImage,omitempty"`
	DesiredDigest string         `json:"desiredDigest,omitempty"`
	ActualImage   string         `json:"actualImage,omitempty"`
	ActualDigest  string         `json:"actualDigest,omitempty"`
	Revision      string         `json:"revision,omitempty"`
	TrafficPct    int32          `json:"trafficPercent,omitempty"`
	Ready         bool           `json:"ready,omitempty"`
	ReadyMessage  string         `json:"readyMessage,omitempty"`
	URL           string         `json:"url,omitempty"`
	ConsoleURL    string         `json:"consoleUrl,omitempty"`
	State         InventoryState `json:"state"`
}

type InventoryResult struct {
	Items []InventoryItem `json:"items"`
}

func (e *Engine) Inventory(ctx context.Context, req InventoryRequest) (*InventoryResult, error) {
	envs, err := e.inventoryEnvs(req.Env)
	if err != nil {
		return nil, err
	}

	result := &InventoryResult{}
	for _, env := range envs {
		managed, err := e.managedServices(env)
		if err != nil {
			return nil, err
		}
		actual, err := e.CloudRun.ListServices(ctx, env)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for _, deployed := range actual {
			seen[deployed.Service] = true
			item := inventoryActualItem(env, deployed)
			if svc, ok := managed[deployed.Service]; ok {
				item.Managed = true
				item.DesiredImage = svc.Image
				if _, digest, err := manifest.SplitDigest(svc.Image); err == nil {
					item.DesiredDigest = digest
				}
				if item.ActualDigest == item.DesiredDigest {
					item.State = InventoryInSync
				} else {
					item.State = InventoryDrift
				}
			}
			result.Items = append(result.Items, item)
		}
		for name, svc := range managed {
			if seen[name] {
				continue
			}
			item := InventoryItem{
				Env:          env.Name,
				Project:      env.GCP.Project,
				Region:       env.GCP.Region,
				Service:      svc.Name,
				Managed:      true,
				DesiredImage: svc.Image,
				State:        InventoryNotDeployed,
			}
			if _, digest, err := manifest.SplitDigest(svc.Image); err == nil {
				item.DesiredDigest = digest
			}
			result.Items = append(result.Items, item)
		}
	}
	sort.Slice(result.Items, func(i, j int) bool {
		if result.Items[i].Env != result.Items[j].Env {
			return result.Items[i].Env < result.Items[j].Env
		}
		return result.Items[i].Service < result.Items[j].Service
	})
	return result, nil
}

func (e *Engine) inventoryEnvs(only string) ([]*manifest.Environment, error) {
	if only != "" {
		env, err := e.Repo.Environment(only)
		if err != nil {
			return nil, err
		}
		return []*manifest.Environment{env}, nil
	}
	envs := make([]*manifest.Environment, 0, len(e.Repo.Config.Environments))
	for _, env := range e.Repo.Config.Environments {
		envs = append(envs, env)
	}
	sort.Slice(envs, func(i, j int) bool { return envs[i].Name < envs[j].Name })
	return envs, nil
}

func (e *Engine) managedServices(env *manifest.Environment) (map[string]*manifest.Service, error) {
	services, err := e.Repo.LoadServices(env)
	if err != nil {
		return nil, err
	}
	managed := make(map[string]*manifest.Service, len(services))
	for _, svc := range services {
		managed[svc.Name] = svc
	}
	return managed, nil
}

func inventoryActualItem(env *manifest.Environment, deployed cloudrun.Deployed) InventoryItem {
	item := InventoryItem{
		Env:          env.Name,
		Project:      env.GCP.Project,
		Region:       env.GCP.Region,
		Service:      deployed.Service,
		ActualImage:  deployed.Image,
		Revision:     deployed.Revision,
		TrafficPct:   deployed.TrafficPercent,
		Ready:        deployed.Ready,
		ReadyMessage: deployed.ReadyMessage,
		URL:          deployed.URL,
		ConsoleURL:   deployed.ConsoleURL,
		State:        InventoryUnmanaged,
	}
	if _, digest, err := manifest.SplitDigest(deployed.Image); err == nil {
		item.ActualDigest = digest
	}
	return item
}

package core

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/Retr0413/wataridori/internal/cloudrun"
	"github.com/Retr0413/wataridori/internal/manifest"
)

// TimelineRequest reconstructs deployment history from Cloud Run revisions.
//
// History (§2.5) answers "what did Wataridori do"; it reads the SQLite store
// and is empty for a service deployed by a CI pipeline. Timeline answers "what
// happened to this service", reading the observed side, so it stays truthful
// no matter who deployed. Comparing two environments on one axis is what makes
// a drift explainable rather than just flagged.
type TimelineRequest struct {
	Env     string `json:"env,omitempty"`     // empty = all environments
	Service string `json:"service,omitempty"` // optional filter
	// Limit bounds the revisions read per service; 0 means DefaultTimelineLimit.
	Limit int `json:"limit,omitempty"`
}

// DefaultTimelineLimit is the per-service revision cap when none is given.
const DefaultTimelineLimit = 20

// TimelineEntry is one revision of one managed service.
type TimelineEntry struct {
	Env string `json:"env"`
	// Service is the manifest identity: two environments' revisions of one
	// service share it even when their Cloud Run names differ.
	Service string `json:"service"`
	// RunName is the Cloud Run service the revision belongs to.
	RunName    string    `json:"runName,omitempty"`
	Revision   string    `json:"revision"`
	Image      string    `json:"image,omitempty"`
	Digest     string    `json:"digest,omitempty"`
	CreateTime time.Time `json:"createTime"`
	Ready      bool      `json:"ready"`
	TrafficPct int32     `json:"trafficPercent"`
	// Current marks the revision serving the largest share of traffic.
	Current bool `json:"current"`
	// Desired marks the revision the Git manifest points at. Current and
	// Desired on different entries is exactly what "drift" means.
	Desired    bool   `json:"desired"`
	ConsoleURL string `json:"consoleUrl,omitempty"`
}

type TimelineResult struct {
	Entries []TimelineEntry `json:"entries"`
}

func (e *Engine) Timeline(ctx context.Context, req TimelineRequest) (*TimelineResult, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = DefaultTimelineLimit
	}

	envs, err := e.inventoryEnvs(req.Env)
	if err != nil {
		return nil, err
	}

	result := &TimelineResult{}
	for _, env := range envs {
		services, err := e.services(env, req.Service)
		if err != nil {
			// A service filter that matches in one environment but not another
			// is normal when environments declare different sets of services;
			// skip that environment rather than failing the whole timeline.
			var unknown *UnknownServiceError
			if errors.As(err, &unknown) {
				continue
			}
			return nil, err
		}
		for _, svc := range services {
			entries, err := e.serviceTimeline(ctx, env, svc, limit)
			if err != nil {
				return nil, err
			}
			result.Entries = append(result.Entries, entries...)
		}
	}

	// Newest first across every environment, so the merged list reads as one
	// axis. Ties fall back to env/service for a stable order.
	sort.SliceStable(result.Entries, func(i, j int) bool {
		a, b := result.Entries[i], result.Entries[j]
		if !a.CreateTime.Equal(b.CreateTime) {
			return a.CreateTime.After(b.CreateTime)
		}
		if a.Env != b.Env {
			return a.Env < b.Env
		}
		return a.Service < b.Service
	})
	return result, nil
}

// serviceTimeline reads one service's revisions and annotates them with the
// manifest's desired digest and the revision currently serving traffic.
func (e *Engine) serviceTimeline(ctx context.Context, env *manifest.Environment, svc *manifest.Service, limit int) ([]TimelineEntry, error) {
	revisions, err := e.CloudRun.ListRevisions(ctx, env, svc.RunName())
	if err != nil {
		return nil, err
	}
	if len(revisions) > limit {
		revisions = revisions[:limit]
	}

	// An unparsable manifest image would already have failed validation, but
	// an empty desired digest simply means nothing is marked desired.
	_, desiredDigest, _ := manifest.SplitDigest(svc.Image)
	current := currentRevision(revisions)

	entries := make([]TimelineEntry, 0, len(revisions))
	for _, rev := range revisions {
		entry := TimelineEntry{
			Env:        env.Name,
			Service:    svc.Name,
			RunName:    svc.RunName(),
			Revision:   rev.Name,
			Image:      rev.Image,
			CreateTime: rev.CreateTime,
			Ready:      rev.Ready,
			TrafficPct: rev.TrafficPercent,
			Current:    rev.Name == current,
			ConsoleURL: cloudrun.ConsoleURL(env, svc.RunName()),
		}
		if _, digest, err := manifest.SplitDigest(rev.Image); err == nil {
			entry.Digest = digest
			entry.Desired = desiredDigest != "" && digest == desiredDigest
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// currentRevision returns the name of the revision with the largest traffic
// share, or "" when nothing serves traffic.
func currentRevision(revisions []cloudrun.Revision) string {
	name := ""
	var best int32
	for _, rev := range revisions {
		if rev.TrafficPercent > best {
			best = rev.TrafficPercent
			name = rev.Name
		}
	}
	return name
}

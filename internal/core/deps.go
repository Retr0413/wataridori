package core

import (
	"context"
	"time"

	"github.com/Retr0413/wataridori/internal/cloudrun"
	"github.com/Retr0413/wataridori/internal/manifest"
	"github.com/Retr0413/wataridori/internal/store"
)

// CloudRun is the slice of internal/cloudrun that the use cases consume.
type CloudRun interface {
	// Get returns the observed state, or (nil, nil) when not deployed.
	Get(ctx context.Context, env *manifest.Environment, name string) (*cloudrun.Deployed, error)
	Apply(ctx context.Context, env *manifest.Environment, svc *manifest.Service, timeout time.Duration) (*cloudrun.Deployed, error)
	ListRevisions(ctx context.Context, env *manifest.Environment, service string) ([]cloudrun.Revision, error)
	SetTraffic(ctx context.Context, env *manifest.Environment, service, revision string) error
}

// ImageCopier copies a digest-pinned image into a destination repository.
type ImageCopier interface {
	Copy(ctx context.Context, srcRef, dstPath string) (copied bool, err error)
}

// Committer records manifest changes as a git commit.
type Committer interface {
	Commit(startDir string, files []string, message string) (commitID string, err error)
}

// History persists and lists operation records.
type History interface {
	Record(ctx context.Context, e store.Entry) error
	List(ctx context.Context, opts store.ListOptions) ([]store.Entry, error)
}

// Engine wires the use cases to their dependencies. Callers (CLI now, the
// Connect RPC server in Phase 2) construct one Engine per invocation.
type Engine struct {
	Repo     *manifest.Repo
	CloudRun CloudRun
	Copier   ImageCopier
	Commit   Committer
	History  History
	// Actor is recorded in history entries (ADC principal or OS user).
	Actor string
}

// DefaultApplyTimeout bounds the wait for a revision to become ready.
const DefaultApplyTimeout = 5 * time.Minute

// services loads the environment's manifests, optionally filtered to one
// service name.
func (e *Engine) services(env *manifest.Environment, only string) ([]*manifest.Service, error) {
	services, err := e.Repo.LoadServices(env)
	if err != nil {
		return nil, err
	}
	if only == "" {
		return services, nil
	}
	for _, svc := range services {
		if svc.Name == only {
			return []*manifest.Service{svc}, nil
		}
	}
	return nil, &UnknownServiceError{Env: env.Name, Service: only}
}

// UnknownServiceError reports a --service filter that matched nothing.
type UnknownServiceError struct {
	Env     string
	Service string
}

func (e *UnknownServiceError) Error() string {
	return "no service named \"" + e.Service + "\" in environment \"" + e.Env + "\""
}

func (e *Engine) record(ctx context.Context, action store.Action, env, service, digest string, detail map[string]string) error {
	return e.History.Record(ctx, store.Entry{
		Actor:   e.Actor,
		Action:  action,
		Env:     env,
		Service: service,
		Digest:  digest,
		Detail:  detail,
	})
}

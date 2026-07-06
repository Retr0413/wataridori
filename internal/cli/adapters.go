package cli

import (
	"context"
	"time"

	"github.com/Retr0413/wataridori/internal/cloudrun"
	"github.com/Retr0413/wataridori/internal/manifest"
)

// cloudRunAdapter narrows *cloudrun.Client to the core.CloudRun interface.
type cloudRunAdapter struct {
	c *cloudrun.Client
}

func (a cloudRunAdapter) Get(ctx context.Context, env *manifest.Environment, name string) (*cloudrun.Deployed, error) {
	return a.c.GetService(ctx, env, name)
}

func (a cloudRunAdapter) Apply(ctx context.Context, env *manifest.Environment, svc *manifest.Service, timeout time.Duration) (*cloudrun.Deployed, error) {
	return a.c.Apply(ctx, env, svc, timeout)
}

func (a cloudRunAdapter) ListRevisions(ctx context.Context, env *manifest.Environment, service string) ([]cloudrun.Revision, error) {
	return a.c.ListRevisions(ctx, env, service)
}

func (a cloudRunAdapter) SetTraffic(ctx context.Context, env *manifest.Environment, service, revision string) error {
	return a.c.SetTraffic(ctx, env, service, revision)
}

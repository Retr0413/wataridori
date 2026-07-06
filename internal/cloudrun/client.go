package cloudrun

import (
	"context"
	"fmt"
	"sort"
	"time"

	run "cloud.google.com/go/run/apiv2"
	runpb "cloud.google.com/go/run/apiv2/runpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Retr0413/wataridori/internal/manifest"
)

// Deployed is the observed state of a service: what is actually serving
// traffic, as opposed to the desired state in the manifest.
type Deployed struct {
	Service string `json:"service"`
	// Image is the image of the revision currently serving the largest
	// share of traffic — after a rollback this differs from the template.
	Image          string `json:"image"`
	Revision       string `json:"revision"`
	TrafficPercent int32  `json:"trafficPercent"`
	Ready          bool   `json:"ready"`
	ReadyMessage   string `json:"readyMessage,omitempty"`
	URL            string `json:"url,omitempty"`
	ConsoleURL     string `json:"consoleUrl,omitempty"`
}

// Revision is one Cloud Run revision, newest first in listings.
type Revision struct {
	Name           string    `json:"name"`
	Image          string    `json:"image"`
	CreateTime     time.Time `json:"createTime"`
	Ready          bool      `json:"ready"`
	TrafficPercent int32     `json:"trafficPercent"`
}

// Client wraps the Cloud Run Admin API v2. Credentials come from ADC.
type Client struct {
	services  *run.ServicesClient
	revisions *run.RevisionsClient
}

func NewClient(ctx context.Context) (*Client, error) {
	services, err := run.NewServicesClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating Cloud Run services client (is ADC configured?): %w", err)
	}
	revisions, err := run.NewRevisionsClient(ctx)
	if err != nil {
		_ = services.Close()
		return nil, err
	}
	return &Client{services: services, revisions: revisions}, nil
}

func (c *Client) Close() error {
	err1 := c.services.Close()
	err2 := c.revisions.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// GetService returns the observed state of a service, or (nil, nil) when
// the service does not exist.
func (c *Client) GetService(ctx context.Context, env *manifest.Environment, name string) (*Deployed, error) {
	svc, err := c.services.GetService(ctx, &runpb.GetServiceRequest{Name: ServiceName(env, name)})
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting service %s: %w", name, err)
	}
	return c.deployed(ctx, env, name, svc)
}

// Apply creates or updates the service to match the manifest and waits
// until the operation finishes (the new revision is ready or failed).
func (c *Client) Apply(ctx context.Context, env *manifest.Environment, svc *manifest.Service, timeout time.Duration) (*Deployed, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	desired := BuildService(env, svc)
	existing, err := c.services.GetService(ctx, &runpb.GetServiceRequest{Name: desired.Name})

	switch {
	case isNotFound(err):
		op, err := c.services.CreateService(ctx, &runpb.CreateServiceRequest{
			Parent:    ParentName(env),
			ServiceId: svc.Name,
			Service:   desired,
		})
		if err != nil {
			return nil, fmt.Errorf("creating service %s: %w", svc.Name, err)
		}
		if _, err := op.Wait(ctx); err != nil {
			return nil, fmt.Errorf("service %s failed to become ready: %w", svc.Name, err)
		}
	case err != nil:
		return nil, fmt.Errorf("getting service %s: %w", svc.Name, err)
	default:
		_ = existing // full replacement: the manifest is the source of truth
		op, err := c.services.UpdateService(ctx, &runpb.UpdateServiceRequest{Service: desired})
		if err != nil {
			return nil, fmt.Errorf("updating service %s: %w", svc.Name, err)
		}
		if _, err := op.Wait(ctx); err != nil {
			return nil, fmt.Errorf("service %s failed to become ready: %w", svc.Name, err)
		}
	}
	return c.GetService(ctx, env, svc.Name)
}

// ListRevisions returns the revisions of a service, newest first.
func (c *Client) ListRevisions(ctx context.Context, env *manifest.Environment, service string) ([]Revision, error) {
	svc, err := c.services.GetService(ctx, &runpb.GetServiceRequest{Name: ServiceName(env, service)})
	if err != nil {
		return nil, fmt.Errorf("getting service %s: %w", service, err)
	}
	traffic := trafficByRevision(svc)

	it := c.revisions.ListRevisions(ctx, &runpb.ListRevisionsRequest{Parent: ServiceName(env, service)})
	var revisions []Revision
	for {
		rev, err := it.Next()
		if err != nil {
			if isIteratorDone(err) {
				break
			}
			return nil, err
		}
		revisions = append(revisions, Revision{
			Name:           shortName(rev.GetName()),
			Image:          containerImage(rev.GetContainers()),
			CreateTime:     rev.GetCreateTime().AsTime(),
			Ready:          conditionReady(rev.GetConditions()),
			TrafficPercent: traffic[shortName(rev.GetName())],
		})
	}
	sort.Slice(revisions, func(i, j int) bool { return revisions[i].CreateTime.After(revisions[j].CreateTime) })
	return revisions, nil
}

// SetTraffic routes 100% of traffic to the named revision.
func (c *Client) SetTraffic(ctx context.Context, env *manifest.Environment, service, revision string) error {
	svc, err := c.services.GetService(ctx, &runpb.GetServiceRequest{Name: ServiceName(env, service)})
	if err != nil {
		return fmt.Errorf("getting service %s: %w", service, err)
	}
	svc.Traffic = []*runpb.TrafficTarget{{
		Type:     runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION,
		Revision: revision,
		Percent:  100,
	}}
	op, err := c.services.UpdateService(ctx, &runpb.UpdateServiceRequest{Service: svc})
	if err != nil {
		return fmt.Errorf("updating traffic of %s: %w", service, err)
	}
	if _, err := op.Wait(ctx); err != nil {
		return fmt.Errorf("switching traffic of %s to %s: %w", service, revision, err)
	}
	return nil
}

// deployed resolves the revision serving the largest traffic share and its
// image, so the observed state reflects rollbacks (which only move traffic).
func (c *Client) deployed(ctx context.Context, env *manifest.Environment, name string, svc *runpb.Service) (*Deployed, error) {
	d := &Deployed{
		Service:      name,
		Ready:        conditionReadyService(svc),
		ReadyMessage: conditionMessage(svc.GetConditions()),
		URL:          svc.GetUri(),
		ConsoleURL:   ConsoleURL(env, name),
	}

	var servingRevision string
	for _, ts := range svc.GetTrafficStatuses() {
		if ts.GetPercent() < d.TrafficPercent {
			continue
		}
		d.TrafficPercent = ts.GetPercent()
		if r := ts.GetRevision(); r != "" {
			servingRevision = r
		} else {
			servingRevision = shortName(svc.GetLatestReadyRevision())
		}
	}
	if servingRevision == "" {
		servingRevision = shortName(svc.GetLatestReadyRevision())
	}
	d.Revision = servingRevision
	if servingRevision == "" {
		return d, nil
	}

	rev, err := c.revisions.GetRevision(ctx, &runpb.GetRevisionRequest{
		Name: ServiceName(env, name) + "/revisions/" + servingRevision,
	})
	if err != nil {
		return nil, fmt.Errorf("getting revision %s: %w", servingRevision, err)
	}
	d.Image = containerImage(rev.GetContainers())
	return d, nil
}

func isNotFound(err error) bool {
	return status.Code(err) == codes.NotFound
}

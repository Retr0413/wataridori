package cloudrun

import (
	"context"
	"fmt"
	"time"

	runpb "cloud.google.com/go/run/apiv2/runpb"

	"github.com/Retr0413/wataridori/internal/manifest"
)

// defaultRequestTimeout is Cloud Run's own default, reported on every service
// whether or not anyone chose it.
const defaultRequestTimeout = 5 * time.Minute

// UnmanagedSettings reports configuration that the running service has and a
// Wataridori manifest cannot express.
//
// Apply replaces the service wholesale (the manifest is the source of truth),
// so anything in this list disappears on the next apply. The manifest schema
// is deliberately small — it is not trying to be Cloud Run's API — which makes
// silent loss the failure mode to guard against, especially for a service
// originally created by Terraform or gcloud.
//
// An empty result means an apply changes only what the manifest describes. A
// service that does not exist yet returns nothing: there is nothing to lose.
func (c *Client) UnmanagedSettings(ctx context.Context, env *manifest.Environment, svc *manifest.Service) ([]string, error) {
	existing, err := c.services.GetService(ctx, &runpb.GetServiceRequest{
		Name: ServiceName(env, svc.RunName()),
	})
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting service %s: %w", svc.RunName(), err)
	}
	return unmanagedSettings(existing, svc), nil
}

func unmanagedSettings(existing *runpb.Service, svc *manifest.Service) []string {
	var found []string
	tmpl := existing.GetTemplate()

	if t := tmpl.GetTimeout(); t != nil && t.AsDuration() != defaultRequestTimeout {
		found = append(found, fmt.Sprintf("request timeout (%s)", t.AsDuration()))
	}
	if tmpl.GetVpcAccess() != nil {
		found = append(found, "VPC access")
	}
	if len(tmpl.GetVolumes()) > 0 {
		found = append(found, fmt.Sprintf("%d volume(s)", len(tmpl.GetVolumes())))
	}
	if tmpl.GetExecutionEnvironment() == runpb.ExecutionEnvironment_EXECUTION_ENVIRONMENT_GEN2 {
		found = append(found, "second-generation execution environment")
	}
	if tmpl.GetSessionAffinity() {
		found = append(found, "session affinity")
	}
	if tmpl.GetEncryptionKey() != "" {
		found = append(found, "customer-managed encryption key")
	}

	containers := tmpl.GetContainers()
	if len(containers) > 1 {
		found = append(found, fmt.Sprintf("%d sidecar container(s)", len(containers)-1))
	}
	if len(containers) > 0 {
		found = append(found, unmanagedContainerSettings(containers[0], svc)...)
	}
	return found
}

func unmanagedContainerSettings(c *runpb.Container, svc *manifest.Service) []string {
	var found []string
	if c.GetStartupProbe() != nil {
		found = append(found, "startup probe")
	}
	if c.GetLivenessProbe() != nil {
		found = append(found, "liveness probe")
	}
	if len(c.GetCommand()) > 0 || len(c.GetArgs()) > 0 {
		found = append(found, "container command/args")
	}
	if len(c.GetVolumeMounts()) > 0 {
		found = append(found, "volume mounts")
	}
	// BuildService only emits ResourceRequirements when the manifest sets
	// resources, and the zero CpuIdle it then sends means "CPU always
	// allocated". A throttled service would be switched over silently.
	if c.GetResources().GetCpuIdle() && (svc.Resources.CPU != "" || svc.Resources.Memory != "") {
		found = append(found, "CPU throttling (apply would switch to CPU always allocated)")
	}
	return found
}

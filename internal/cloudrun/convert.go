package cloudrun

import (
	"fmt"

	runpb "cloud.google.com/go/run/apiv2/runpb"

	"github.com/Retr0413/wataridori/internal/manifest"
)

const defaultPort = 8080

// ParentName returns the location resource name of an environment.
func ParentName(env *manifest.Environment) string {
	return fmt.Sprintf("projects/%s/locations/%s", env.GCP.Project, env.GCP.Region)
}

// ServiceName returns the full service resource name.
func ServiceName(env *manifest.Environment, service string) string {
	return fmt.Sprintf("%s/services/%s", ParentName(env), service)
}

// envVar maps a manifest variable to the Cloud Run one-of: a literal value or
// a Secret Manager reference. Both are sent on every apply — apply replaces
// the service wholesale, so anything omitted here disappears from the running
// revision.
func envVar(e manifest.EnvVar) *runpb.EnvVar {
	if e.Secret != "" {
		return &runpb.EnvVar{
			Name: e.Name,
			Values: &runpb.EnvVar_ValueSource{ValueSource: &runpb.EnvVarSource{
				SecretKeyRef: &runpb.SecretKeySelector{
					Secret:  e.Secret,
					Version: e.SecretVersion(),
				},
			}},
		}
	}
	return &runpb.EnvVar{Name: e.Name, Values: &runpb.EnvVar_Value{Value: e.Value}}
}

// ConsoleURL deep-links to the Cloud Console page of a service.
func ConsoleURL(env *manifest.Environment, service string) string {
	return fmt.Sprintf("https://console.cloud.google.com/run/detail/%s/%s/revisions?project=%s",
		env.GCP.Region, service, env.GCP.Project)
}

// BuildService converts a service manifest into a Cloud Run v2 Service.
// Only the fields defined in docs/spec/phase1-cli.md §1.2 are populated;
// everything else keeps the API defaults.
func BuildService(env *manifest.Environment, svc *manifest.Service) *runpb.Service {
	port := svc.Port
	if port == 0 {
		port = defaultPort
	}

	container := &runpb.Container{
		Image: svc.Image,
		Ports: []*runpb.ContainerPort{{ContainerPort: port}},
	}
	for _, e := range svc.Env {
		container.Env = append(container.Env, envVar(e))
	}
	limits := map[string]string{}
	if svc.Resources.CPU != "" {
		limits["cpu"] = svc.Resources.CPU
	}
	if svc.Resources.Memory != "" {
		limits["memory"] = svc.Resources.Memory
	}
	if len(limits) > 0 {
		container.Resources = &runpb.ResourceRequirements{Limits: limits}
	}

	template := &runpb.RevisionTemplate{
		Containers:     []*runpb.Container{container},
		ServiceAccount: svc.ServiceAccount,
		Scaling: &runpb.RevisionScaling{
			MinInstanceCount: svc.Scaling.Min,
			MaxInstanceCount: svc.Scaling.Max,
		},
	}
	if svc.Concurrency > 0 {
		template.MaxInstanceRequestConcurrency = svc.Concurrency
	}

	return &runpb.Service{
		Name:     ServiceName(env, svc.RunName()),
		Template: template,
		// All traffic to the latest ready revision. Rollback overrides this
		// with a pinned revision; a later apply intentionally resets it.
		Traffic: []*runpb.TrafficTarget{{
			Type:    runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST,
			Percent: 100,
		}},
	}
}

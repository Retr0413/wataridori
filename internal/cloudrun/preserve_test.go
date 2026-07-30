package cloudrun

import (
	"strings"
	"testing"
	"time"

	runpb "cloud.google.com/go/run/apiv2/runpb"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/Retr0413/wataridori/internal/manifest"
)

func service(tmpl *runpb.RevisionTemplate) *runpb.Service {
	return &runpb.Service{Template: tmpl}
}

func container(c *runpb.Container) *runpb.RevisionTemplate {
	return &runpb.RevisionTemplate{Containers: []*runpb.Container{c}}
}

func has(t *testing.T, got []string, want string) {
	t.Helper()
	for _, g := range got {
		if strings.Contains(g, want) {
			return
		}
	}
	t.Errorf("settings = %v, want one containing %q", got, want)
}

// A service whose configuration the manifest fully describes must not be
// flagged, or the guard becomes noise that everyone forces past.
func TestUnmanagedSettingsSilentOnAManifestShapedService(t *testing.T) {
	svc := &manifest.Service{Name: "api", Resources: manifest.Resources{CPU: "1"}}
	existing := service(container(&runpb.Container{
		Image:     "reg.example/api@sha256:abc",
		Ports:     []*runpb.ContainerPort{{ContainerPort: 8080}},
		Resources: &runpb.ResourceRequirements{Limits: map[string]string{"cpu": "1"}},
	}))
	existing.Template.Timeout = durationpb.New(defaultRequestTimeout)

	if got := unmanagedSettings(existing, svc); len(got) != 0 {
		t.Fatalf("settings = %v, want none", got)
	}
}

func TestUnmanagedSettingsFindsProbesAndTimeout(t *testing.T) {
	svc := &manifest.Service{Name: "api"}
	existing := service(container(&runpb.Container{
		StartupProbe:  &runpb.Probe{},
		LivenessProbe: &runpb.Probe{},
	}))
	existing.Template.Timeout = durationpb.New(2 * time.Minute)

	got := unmanagedSettings(existing, svc)
	has(t, got, "startup probe")
	has(t, got, "liveness probe")
	has(t, got, "request timeout (2m0s)")
}

func TestUnmanagedSettingsFindsTemplateLevelFeatures(t *testing.T) {
	svc := &manifest.Service{Name: "api"}
	existing := service(&runpb.RevisionTemplate{
		Containers: []*runpb.Container{
			{VolumeMounts: []*runpb.VolumeMount{{Name: "secrets"}}},
			{Name: "sidecar"},
		},
		Volumes:              []*runpb.Volume{{Name: "secrets"}},
		VpcAccess:            &runpb.VpcAccess{Connector: "projects/p/locations/l/connectors/c"},
		SessionAffinity:      true,
		ExecutionEnvironment: runpb.ExecutionEnvironment_EXECUTION_ENVIRONMENT_GEN2,
	})

	got := unmanagedSettings(existing, svc)
	has(t, got, "VPC access")
	has(t, got, "1 volume(s)")
	has(t, got, "volume mounts")
	has(t, got, "1 sidecar container(s)")
	has(t, got, "session affinity")
	has(t, got, "second-generation")
}

// CpuIdle only flips when the manifest declares resources, because that is
// the only case where BuildService sends ResourceRequirements at all.
func TestUnmanagedSettingsFlagsCPUThrottlingOnlyWithResources(t *testing.T) {
	throttled := func() *runpb.Service {
		return service(container(&runpb.Container{
			Resources: &runpb.ResourceRequirements{CpuIdle: true},
		}))
	}

	withResources := &manifest.Service{Name: "api", Resources: manifest.Resources{Memory: "512Mi"}}
	has(t, unmanagedSettings(throttled(), withResources), "CPU throttling")

	withoutResources := &manifest.Service{Name: "api"}
	if got := unmanagedSettings(throttled(), withoutResources); len(got) != 0 {
		t.Errorf("settings = %v, want none when the manifest declares no resources", got)
	}
}

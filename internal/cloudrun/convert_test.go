package cloudrun

import (
	"testing"

	runpb "cloud.google.com/go/run/apiv2/runpb"

	"github.com/Retr0413/wataridori/internal/manifest"
)

func env() *manifest.Environment {
	return &manifest.Environment{
		Name: "prod",
		GCP:  manifest.GCP{Project: "my-app-prod", Region: "asia-northeast1"},
	}
}

func TestBuildServiceFull(t *testing.T) {
	svc := BuildService(env(), &manifest.Service{
		Name:           "my-app",
		Image:          "img@sha256:abc",
		Env:            []manifest.EnvVar{{Name: "LOG_LEVEL", Value: "warn"}},
		Resources:      manifest.Resources{CPU: "2", Memory: "1Gi"},
		Scaling:        manifest.Scaling{Min: 1, Max: 50},
		ServiceAccount: "sa@my-app-prod.iam.gserviceaccount.com",
		Concurrency:    80,
		Port:           9000,
	})

	if svc.Name != "projects/my-app-prod/locations/asia-northeast1/services/my-app" {
		t.Errorf("Name = %q", svc.Name)
	}
	c := svc.Template.Containers[0]
	if c.Image != "img@sha256:abc" {
		t.Errorf("Image = %q", c.Image)
	}
	if len(c.Env) != 1 || c.Env[0].Name != "LOG_LEVEL" || c.Env[0].GetValue() != "warn" {
		t.Errorf("Env = %+v", c.Env)
	}
	if c.Resources.Limits["cpu"] != "2" || c.Resources.Limits["memory"] != "1Gi" {
		t.Errorf("Limits = %v", c.Resources.Limits)
	}
	if c.Ports[0].ContainerPort != 9000 {
		t.Errorf("Port = %d", c.Ports[0].ContainerPort)
	}
	tpl := svc.Template
	if tpl.ServiceAccount != "sa@my-app-prod.iam.gserviceaccount.com" {
		t.Errorf("ServiceAccount = %q", tpl.ServiceAccount)
	}
	if tpl.Scaling.MinInstanceCount != 1 || tpl.Scaling.MaxInstanceCount != 50 {
		t.Errorf("Scaling = %+v", tpl.Scaling)
	}
	if tpl.MaxInstanceRequestConcurrency != 80 {
		t.Errorf("Concurrency = %d", tpl.MaxInstanceRequestConcurrency)
	}
	if len(svc.Traffic) != 1 || svc.Traffic[0].Type != runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST ||
		svc.Traffic[0].Percent != 100 {
		t.Errorf("Traffic = %+v", svc.Traffic)
	}
}

// Every apply is a full replacement, so a secret-backed variable has to
// survive the round trip as a reference — dropping it would strip the secret
// off the running service.
func TestBuildServiceBindsSecretEnvVars(t *testing.T) {
	svc := BuildService(env(), &manifest.Service{
		Name:  "my-app",
		Image: "img@sha256:abc",
		Env: []manifest.EnvVar{
			{Name: "LOG_LEVEL", Value: "warn"},
			{Name: "JWT_SECRET", Secret: "my-app-jwt-prod"},
			{Name: "PINNED", Secret: "my-app-pinned", Version: "4"},
		},
	})

	vars := svc.Template.Containers[0].Env
	if len(vars) != 3 {
		t.Fatalf("Env = %+v, want 3 entries", vars)
	}
	if vars[0].GetValue() != "warn" {
		t.Errorf("literal value = %q, want warn", vars[0].GetValue())
	}
	ref := vars[1].GetValueSource().GetSecretKeyRef()
	if ref.GetSecret() != "my-app-jwt-prod" || ref.GetVersion() != "latest" {
		t.Errorf("secret ref = %+v, want my-app-jwt-prod@latest", ref)
	}
	if v := vars[2].GetValueSource().GetSecretKeyRef().GetVersion(); v != "4" {
		t.Errorf("pinned version = %q, want 4", v)
	}
}

// The Cloud Run resource name follows cloudRunName; the manifest's own name
// is only an identity for promotion and the UI.
func TestBuildServiceUsesCloudRunName(t *testing.T) {
	svc := BuildService(env(), &manifest.Service{
		Name: "my-app", CloudRunName: "my-app-prod", Image: "img@sha256:abc",
	})
	if want := "projects/my-app-prod/locations/asia-northeast1/services/my-app-prod"; svc.Name != want {
		t.Errorf("Name = %q, want %q", svc.Name, want)
	}
}

func TestBuildServiceDefaults(t *testing.T) {
	svc := BuildService(env(), &manifest.Service{Name: "min", Image: "img@sha256:abc"})
	c := svc.Template.Containers[0]
	if c.Ports[0].ContainerPort != 8080 {
		t.Errorf("default port = %d, want 8080", c.Ports[0].ContainerPort)
	}
	if c.Resources != nil {
		t.Errorf("Resources should be unset, got %+v", c.Resources)
	}
	if svc.Template.MaxInstanceRequestConcurrency != 0 {
		t.Error("Concurrency should keep API default (0 = unset)")
	}
}

func TestTrafficByRevision(t *testing.T) {
	svc := &runpb.Service{
		LatestReadyRevision: "projects/p/locations/l/services/s/revisions/s-00042",
		TrafficStatuses: []*runpb.TrafficTargetStatus{
			{Revision: "s-00041", Percent: 30},
			{Percent: 70}, // LATEST target has no revision name
		},
	}
	traffic := trafficByRevision(svc)
	if traffic["s-00041"] != 30 || traffic["s-00042"] != 70 {
		t.Errorf("traffic = %v", traffic)
	}
}

func TestShortName(t *testing.T) {
	if shortName("projects/p/locations/l/services/s/revisions/s-00042") != "s-00042" {
		t.Error("shortName failed on full resource name")
	}
	if shortName("s-00042") != "s-00042" {
		t.Error("shortName failed on bare name")
	}
}

package cloudrun

import (
	"testing"

	runpb "cloud.google.com/go/run/apiv2/runpb"
	"google.golang.org/protobuf/proto"
)

func TestBuildImageOnlyUpdatePreservesObservedService(t *testing.T) {
	oldImage := "reg.example/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	newImage := "reg.example/app@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	existing := &runpb.Service{
		Name:   "projects/p/locations/r/services/app",
		Labels: map[string]string{"terraform": "owned"},
		Template: &runpb.RevisionTemplate{Containers: []*runpb.Container{
			{Name: "app", Image: oldImage, Args: []string{"serve"}},
			{Name: "sidecar", Image: oldImage, Args: []string{"proxy"}},
		}},
	}
	original := proto.Clone(existing).(*runpb.Service)

	req, err := BuildImageOnlyUpdate(existing, newImage)
	if err != nil {
		t.Fatal(err)
	}
	if got := req.GetUpdateMask().GetPaths(); len(got) != 1 || got[0] != "template.containers" {
		t.Fatalf("update mask = %v", got)
	}
	if !proto.Equal(existing, original) {
		t.Fatal("input service was mutated")
	}
	if req.Service.Template.Containers[0].Image != newImage {
		t.Fatalf("primary image = %q", req.Service.Template.Containers[0].Image)
	}
	if req.Service.Template.Containers[1].Image != oldImage || req.Service.Labels["terraform"] != "owned" {
		t.Fatal("non-image configuration was not preserved")
	}
}

func TestBuildImageOnlyUpdateRequiresExistingContainer(t *testing.T) {
	_, err := BuildImageOnlyUpdate(&runpb.Service{}, "reg.example/app@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err == nil {
		t.Fatal("expected missing container error")
	}
}

func TestBuildTrafficUpdateOwnsOnlyTraffic(t *testing.T) {
	existing := &runpb.Service{Name: "projects/p/locations/r/services/app", Labels: map[string]string{"terraform": "owned"}}
	original := proto.Clone(existing).(*runpb.Service)
	req, err := BuildTrafficUpdate(existing, "app-00002")
	if err != nil {
		t.Fatal(err)
	}
	if got := req.GetUpdateMask().GetPaths(); len(got) != 1 || got[0] != "traffic" {
		t.Fatalf("update mask = %v", got)
	}
	if !proto.Equal(existing, original) {
		t.Fatal("input service was mutated")
	}
	if len(req.Service.Traffic) != 1 || req.Service.Traffic[0].Revision != "app-00002" || req.Service.Traffic[0].Percent != 100 {
		t.Fatalf("traffic = %+v", req.Service.Traffic)
	}
}

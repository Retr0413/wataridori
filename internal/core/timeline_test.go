package core

import (
	"context"
	"testing"
	"time"

	"github.com/Retr0413/wataridori/internal/cloudrun"
)

var timelineBase = time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

// seedTimeline gives dev two revisions (the newest one not the manifest's, so
// dev has drifted) and prod one revision matching its manifest.
func seedTimeline(e *testEngine) {
	e.cloudRun.revisions["dev/my-app"] = []cloudrun.Revision{
		{Name: "my-app-00003", Image: "reg.example/dev/my-app@" + digestOld,
			CreateTime: timelineBase, Ready: true, TrafficPercent: 100},
		{Name: "my-app-00002", Image: "reg.example/dev/my-app@" + digestNew,
			CreateTime: timelineBase.Add(-48 * time.Hour), Ready: true},
	}
	e.cloudRun.revisions["prod/my-app"] = []cloudrun.Revision{
		{Name: "my-app-00001", Image: "reg.example/prod/my-app@" + digestOld,
			CreateTime: timelineBase.Add(-24 * time.Hour), Ready: true, TrafficPercent: 100},
	}
}

func TestTimelineMergesEnvironmentsNewestFirst(t *testing.T) {
	e := newTestEngine(t, false)
	seedTimeline(e)

	res, err := e.Timeline(context.Background(), TimelineRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(res.Entries))
	}

	want := []string{"my-app-00003", "my-app-00001", "my-app-00002"}
	for i, name := range want {
		if res.Entries[i].Revision != name {
			t.Errorf("entry %d revision = %q, want %q", i, res.Entries[i].Revision, name)
		}
	}
	// The middle entry comes from prod: environments interleave on one axis.
	if res.Entries[1].Env != "prod" {
		t.Errorf("entry 1 env = %q, want prod", res.Entries[1].Env)
	}
}

func TestTimelineMarksCurrentAndDesired(t *testing.T) {
	e := newTestEngine(t, false)
	seedTimeline(e)

	res, err := e.Timeline(context.Background(), TimelineRequest{Env: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(res.Entries))
	}

	// dev's manifest pins digestNew, but 00003 (digestOld) serves traffic:
	// current and desired land on different revisions, which is the drift.
	serving, inGit := res.Entries[0], res.Entries[1]
	if !serving.Current || serving.Desired {
		t.Errorf("serving revision: current=%v desired=%v, want true/false", serving.Current, serving.Desired)
	}
	if inGit.Current || !inGit.Desired {
		t.Errorf("manifest revision: current=%v desired=%v, want false/true", inGit.Current, inGit.Desired)
	}
	if serving.Digest != digestOld {
		t.Errorf("serving digest = %q, want %q", serving.Digest, digestOld)
	}
	if serving.ConsoleURL == "" {
		t.Error("console URL is empty")
	}
}

func TestTimelineLimitsRevisionsPerService(t *testing.T) {
	e := newTestEngine(t, false)
	seedTimeline(e)

	res, err := e.Timeline(context.Background(), TimelineRequest{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	// One per service, not one overall.
	if len(res.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(res.Entries))
	}
}

func TestTimelineUnknownEnvErrors(t *testing.T) {
	e := newTestEngine(t, false)

	_, err := e.Timeline(context.Background(), TimelineRequest{Env: "staging"})
	mustContain(t, err, `unknown environment "staging"`)
}

// A service filter that only matches in one environment narrows the timeline
// instead of failing: environments may declare different sets of services.
func TestTimelineSkipsEnvironmentsWithoutTheService(t *testing.T) {
	e := newTestEngine(t, false)
	seedTimeline(e)

	res, err := e.Timeline(context.Background(), TimelineRequest{Service: "other-app"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(res.Entries))
	}
}

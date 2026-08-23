package core

import (
	"context"
	"testing"
	"time"
)

func TestRecordImageEventUpdatesAutoManifestAndIsIdempotent(t *testing.T) {
	e := newTestEngine(t, false)
	req := ImageEventRequest{
		Env: "dev", Service: "my-app", Image: "reg.example/dev/my-app@" + digestOld,
		OccurredAt: time.Now().UTC(), MaxAge: time.Hour,
		Provenance: Provenance{
			SourceRepository: "Retr0413/BrickLog",
			SourceCommit:     "1234567890abcdef1234567890abcdef12345678",
			WorkflowRun:      "https://github.com/Retr0413/BrickLog/actions/runs/123",
		},
	}
	first, err := e.RecordImageEvent(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := e.RecordImageEvent(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Changed || second.Changed || first.EventID != second.EventID {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	if len(e.copier.verified) != 2 {
		t.Fatalf("registry verifications = %d", len(e.copier.verified))
	}
}

func TestRecordImageEventRejectsExpiredEvent(t *testing.T) {
	e := newTestEngine(t, false)
	_, err := e.RecordImageEvent(context.Background(), ImageEventRequest{
		Env: "dev", Service: "my-app", Image: "reg.example/dev/my-app@" + digestOld,
		OccurredAt: time.Now().Add(-2 * time.Hour), MaxAge: time.Hour,
		Provenance: Provenance{
			SourceRepository: "Retr0413/BrickLog",
			SourceCommit:     "1234567890abcdef1234567890abcdef12345678",
			WorkflowRun:      "https://github.com/Retr0413/BrickLog/actions/runs/123",
		},
	})
	mustContain(t, err, "expired")
}

func TestRecordImageEventRejectsManualEnvironmentBeforeRegistry(t *testing.T) {
	e := newTestEngine(t, false)
	_, err := e.RecordImageEvent(context.Background(), ImageEventRequest{
		Env: "prod", Service: "my-app", Image: "reg.example/prod/my-app@" + digestNew,
		OccurredAt: time.Now(), MaxAge: time.Hour,
		Provenance: Provenance{
			SourceRepository: "Retr0413/BrickLog",
			SourceCommit:     "1234567890abcdef1234567890abcdef12345678",
			WorkflowRun:      "https://github.com/Retr0413/BrickLog/actions/runs/123",
		},
	})
	mustContain(t, err, `policy "auto" is required`)
	if len(e.copier.verified) != 0 {
		t.Fatal("manual environment must be rejected before registry access")
	}
}

package domain

import (
	"testing"
	"time"
)

func TestRecoveryRunTransition(t *testing.T) {
	now := time.Date(2026, time.September, 13, 12, 0, 0, 0, time.UTC)
	run, err := NewRecoveryRun("recovery-1", testScope(now), testPolicy(), now)
	if err != nil {
		t.Fatalf("NewRecoveryRun() error = %v", err)
	}

	if err := run.Transition(StateFrozen, now.Add(time.Second)); err != nil {
		t.Fatalf("Transition() error = %v", err)
	}
	if run.State != StateFrozen {
		t.Fatalf("State = %q, want %q", run.State, StateFrozen)
	}
	if run.Version != 2 {
		t.Fatalf("Version = %d, want 2", run.Version)
	}
}

func TestRecoveryRunRejectsSkippedAuthorization(t *testing.T) {
	now := time.Date(2026, time.September, 13, 12, 0, 0, 0, time.UTC)
	run, err := NewRecoveryRun("recovery-1", testScope(now), testPolicy(), now)
	if err != nil {
		t.Fatalf("NewRecoveryRun() error = %v", err)
	}

	if err := run.Transition(StateMitigating, now.Add(time.Second)); err == nil {
		t.Fatal("Transition() error = nil, want invalid transition")
	}
}

func testScope(now time.Time) Scope {
	return Scope{
		Repository:      "example/repository",
		Environment:     "prod",
		Service:         "api",
		Digest:          "sha256:abc",
		PlanFingerprint: "plan-1",
		TraceID:         "trace-1",
		FailureStage:    "revision-not-ready",
		ObservedAt:      now,
	}
}

func testPolicy() PolicySnapshot {
	return PolicySnapshot{
		Version:        1,
		Commit:         "policy-commit",
		Mode:           RecoveryAutonomous,
		AllowedActions: []Action{ActionWait, ActionRollback},
		Budget: Budget{
			MaxSteps:         3,
			MaxToolCalls:     8,
			MaxEvidenceBytes: 1024,
			MaxDuration:      5 * time.Minute,
		},
	}
}

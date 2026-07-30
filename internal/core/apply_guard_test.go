package core

import (
	"context"
	"errors"
	"testing"
)

func TestApplyRefusesToDropUnmanagedSettings(t *testing.T) {
	e := newTestEngine(t, false)
	e.cloudRun.unmanaged["prod/my-app"] = []string{"startup probe", "request timeout (2m0s)"}

	_, err := e.Apply(context.Background(), ApplyRequest{Env: "prod"})

	var unmanaged *UnmanagedSettingsError
	if !errors.As(err, &unmanaged) {
		t.Fatalf("error = %v, want *UnmanagedSettingsError", err)
	}
	mustContain(t, err, "startup probe")
	mustContain(t, err, "--force")
	if len(e.cloudRun.applied) != 0 {
		t.Errorf("applied %v; the refusal must happen before any write", e.cloudRun.applied)
	}
}

func TestApplyForceProceedsAndStillReportsWhatItDropped(t *testing.T) {
	e := newTestEngine(t, false)
	e.cloudRun.unmanaged["prod/my-app"] = []string{"startup probe"}

	res, err := e.Apply(context.Background(), ApplyRequest{Env: "prod", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.cloudRun.applied) != 1 {
		t.Fatalf("applied = %v, want one service", e.cloudRun.applied)
	}
	if got := res.Services[0].Unmanaged; len(got) != 1 || got[0] != "startup probe" {
		t.Errorf("unmanaged = %v, want [startup probe]; forcing must not hide the loss", got)
	}
}

// A dry-run is how you find out before committing to anything, so it reports
// rather than refuses.
func TestApplyDryRunReportsUnmanagedSettingsWithoutFailing(t *testing.T) {
	e := newTestEngine(t, false)
	e.cloudRun.unmanaged["prod/my-app"] = []string{"VPC access"}

	res, err := e.Apply(context.Background(), ApplyRequest{Env: "prod", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Services[0].Unmanaged; len(got) != 1 || got[0] != "VPC access" {
		t.Errorf("unmanaged = %v, want [VPC access]", got)
	}
	if len(e.cloudRun.applied) != 0 {
		t.Errorf("applied %v on a dry run", e.cloudRun.applied)
	}
}

func TestApplyProceedsWhenNothingWouldBeDropped(t *testing.T) {
	e := newTestEngine(t, false)

	if _, err := e.Apply(context.Background(), ApplyRequest{Env: "prod"}); err != nil {
		t.Fatal(err)
	}
	if len(e.cloudRun.applied) != 1 {
		t.Fatalf("applied = %v, want one service", e.cloudRun.applied)
	}
}

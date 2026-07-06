package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRecordAndList(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "nested", "history.db"))
	if err != nil {
		t.Fatalf("Open (with missing parent dir): %v", err)
	}
	defer func() { _ = s.Close() }()

	entries := []Entry{
		{Actor: "a@example.com", Action: ActionApply, Env: "dev", Service: "my-app", Digest: "sha256:aaa"},
		{Actor: "a@example.com", Action: ActionPromote, Env: "prod", Service: "my-app", Digest: "sha256:aaa",
			Detail: map[string]string{"from": "dev", "commit": "1a2b3c"}},
		{Actor: "b@example.com", Action: ActionRollback, Env: "prod", Service: "my-app", Digest: "sha256:bbb"},
	}
	for _, e := range entries {
		if err := s.Record(ctx, e); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	all, err := s.List(ctx, ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("want 3 entries, got %d", len(all))
	}
	if all[0].Action != ActionRollback || all[2].Action != ActionApply {
		t.Errorf("not newest-first: %+v", all)
	}
	if all[0].Time.IsZero() {
		t.Error("zero Time not defaulted")
	}

	prod, err := s.List(ctx, ListOptions{Env: "prod", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(prod) != 1 || prod[0].Env != "prod" || prod[0].Action != ActionRollback {
		t.Errorf("filtered list wrong: %+v", prod)
	}

	promote, err := s.List(ctx, ListOptions{Env: "prod", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(promote) != 2 || promote[1].Detail["commit"] != "1a2b3c" {
		t.Errorf("detail roundtrip failed: %+v", promote)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	for range 2 {
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		_ = s.Close()
	}
}

func TestDefaultPathEnvOverride(t *testing.T) {
	t.Setenv("WATARIDORI_DB", "/tmp/custom.db")
	p, err := DefaultPath()
	if err != nil || p != "/tmp/custom.db" {
		t.Errorf("DefaultPath = %q, %v", p, err)
	}
}

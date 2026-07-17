package core

import (
	"strings"
	"testing"

	"github.com/Retr0413/wataridori/internal/manifest"
)

// env builds one environment; promoteFrom is optional.
func env(name, promoteFrom string) *manifest.Environment {
	return &manifest.Environment{Name: name, PromoteFrom: promoteFrom}
}

func envMap(envs ...*manifest.Environment) map[string]*manifest.Environment {
	m := make(map[string]*manifest.Environment, len(envs))
	for _, e := range envs {
		m[e.Name] = e
	}
	return m
}

func names(envs []*manifest.Environment) string {
	out := make([]string, 0, len(envs))
	for _, e := range envs {
		out = append(out, e.Name)
	}
	return strings.Join(out, ",")
}

func TestOrderEnvironments(t *testing.T) {
	tests := []struct {
		name string
		envs map[string]*manifest.Environment
		want string
	}{{
		name: "promotion source precedes its target",
		envs: envMap(env("prod", "dev"), env("dev", "")),
		want: "dev,prod",
	}, {
		name: "chain of three",
		envs: envMap(env("prod", "staging"), env("dev", ""), env("staging", "dev")),
		want: "dev,staging,prod",
	}, {
		name: "unrelated environments sort by name",
		envs: envMap(env("prod", ""), env("dev", "")),
		want: "dev,prod",
	}, {
		name: "one source fanning out to two targets",
		envs: envMap(env("eu-prod", "dev"), env("us-prod", "dev"), env("dev", "")),
		want: "dev,eu-prod,us-prod",
	}, {
		// The validator only checks promoteFrom for policy:manual, so an
		// auto environment can carry a dangling reference.
		name: "unresolvable promoteFrom is ignored",
		envs: envMap(env("dev", "nonexistent"), env("prod", "dev")),
		want: "dev,prod",
	}, {
		// The validator rejects self-reference, but check we do not hang.
		name: "self reference is ignored",
		envs: envMap(env("dev", "dev")),
		want: "dev",
	}, {
		// The validator does not reject cycles; every environment must
		// still come back exactly once.
		name: "cycle falls back to name order",
		envs: envMap(env("a", "b"), env("b", "a")),
		want: "a,b",
	}, {
		name: "cycle does not swallow the acyclic part",
		envs: envMap(env("a", "b"), env("b", "a"), env("dev", ""), env("prod", "dev")),
		want: "dev,prod,a,b",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := names(orderEnvironments(tt.envs)); got != tt.want {
				t.Errorf("order = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListEnvironmentsReportsConfig(t *testing.T) {
	e := newTestEngine(t, false)
	res, err := e.ListEnvironments(ctx, ListEnvironmentsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Environments) != 2 {
		t.Fatalf("environments = %+v, want 2", res.Environments)
	}
	dev, prod := res.Environments[0], res.Environments[1]
	if dev.Name != "dev" || prod.Name != "prod" {
		t.Fatalf("order = %s,%s, want dev,prod", dev.Name, prod.Name)
	}
	if dev.Policy != "auto" || dev.Branch != "develop" {
		t.Errorf("dev = %+v", dev)
	}
	if prod.Policy != "manual" || prod.PromoteFrom != "dev" {
		t.Errorf("prod = %+v", prod)
	}
	if prod.Project != "p-prod" || prod.Region != "asia-northeast1" {
		t.Errorf("prod gcp = %+v", prod)
	}
	if prod.ImageCopyTo != "" {
		t.Errorf("ImageCopyTo = %q, want empty without an imageCopy block", prod.ImageCopyTo)
	}
}

func TestListEnvironmentsReportsImageCopy(t *testing.T) {
	e := newTestEngine(t, true)
	res, err := e.ListEnvironments(ctx, ListEnvironmentsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Environments[1].ImageCopyTo; got != "reg.example/prod" {
		t.Errorf("ImageCopyTo = %q, want reg.example/prod", got)
	}
}

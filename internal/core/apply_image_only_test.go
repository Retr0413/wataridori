package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Retr0413/wataridori/internal/cloudrun"
	"github.com/Retr0413/wataridori/internal/manifest"
)

func TestApplyImageOnlyUsesNarrowCloudRunOperation(t *testing.T) {
	e := newTestEngine(t, false)
	path := filepath.Join(e.Repo.Root, "envs/dev/my-app.yaml")
	if err := os.WriteFile(path, []byte("name: my-app\napplyMode: image-only\nimage: reg.example/dev/my-app@"+digestNew+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo, _, err := manifest.Load(e.Repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	e.Repo = repo
	e.cloudRun.deployed["dev/my-app"] = &cloudrun.Deployed{Service: "my-app", Image: "reg.example/dev/my-app@" + digestOld}
	e.cloudRun.unmanaged["dev/my-app"] = []string{"vpc access", "startup probe"}

	res, err := e.Apply(context.Background(), ApplyRequest{Env: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if got := e.cloudRun.applyMode["dev/my-app"]; got != manifest.ApplyModeImageOnly {
		t.Fatalf("apply mode = %q", got)
	}
	if len(res.Services[0].Unmanaged) != 0 {
		t.Fatalf("image-only apply reported removable settings: %v", res.Services[0].Unmanaged)
	}
}

func TestApplyImageOnlyRefusesMissingService(t *testing.T) {
	e := newTestEngine(t, false)
	path := filepath.Join(e.Repo.Root, "envs/dev/my-app.yaml")
	if err := os.WriteFile(path, []byte("name: my-app\napplyMode: image-only\nimage: reg.example/dev/my-app@"+digestNew+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo, _, err := manifest.Load(e.Repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	e.Repo = repo
	_, err = e.Apply(context.Background(), ApplyRequest{Env: "dev"})
	mustContain(t, err, "create it with Terraform first")
}

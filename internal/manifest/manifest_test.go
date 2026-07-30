package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func writeRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLoadValid(t *testing.T) {
	repo, warnings, err := Load("testdata/valid")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	prod, err := repo.Environment("prod")
	if err != nil {
		t.Fatal(err)
	}
	if prod.Name != "prod" || prod.Policy != PolicyManual || prod.PromoteFrom != "dev" {
		t.Errorf("prod loaded wrong: %+v", prod)
	}
	if prod.ImageCopy == nil || prod.ImageCopy.To != "asia-northeast1-docker.pkg.dev/my-app-prod/images" {
		t.Errorf("prod.ImageCopy loaded wrong: %+v", prod.ImageCopy)
	}

	services, err := repo.LoadServices(prod)
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 1 {
		t.Fatalf("want 1 service, got %d", len(services))
	}
	svc := services[0]
	if svc.Name != "my-app" || svc.Concurrency != 80 || svc.Port != 8080 ||
		svc.Scaling.Min != 1 || svc.Scaling.Max != 50 ||
		svc.Resources.Memory != "1Gi" || len(svc.Env) != 1 {
		t.Errorf("service loaded wrong: %+v", svc)
	}
	if svc.File != filepath.Join("environments/prod", "my-app.yaml") {
		t.Errorf("svc.File = %q", svc.File)
	}
}

func TestLoadEnvironmentErrors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "unsupported version",
			yaml:    "version: 2\nenvironments:\n  dev: {policy: manual, gcp: {project: p, region: r}, services: d}\n",
			wantErr: "unsupported version",
		},
		{
			name:    "no environments",
			yaml:    "version: 1\n",
			wantErr: "no environments",
		},
		{
			name:    "auto without branch",
			yaml:    "version: 1\nenvironments:\n  dev: {policy: auto, gcp: {project: p, region: r}, services: d}\n",
			wantErr: `requires "branch"`,
		},
		{
			name:    "unknown policy",
			yaml:    "version: 1\nenvironments:\n  dev: {policy: rolling, gcp: {project: p, region: r}, services: d}\n",
			wantErr: "unknown policy",
		},
		{
			name:    "missing policy",
			yaml:    "version: 1\nenvironments:\n  dev: {gcp: {project: p, region: r}, services: d}\n",
			wantErr: `"policy" is required`,
		},
		{
			name:    "missing gcp",
			yaml:    "version: 1\nenvironments:\n  dev: {policy: manual, services: d}\n",
			wantErr: `"gcp.project" and "gcp.region" are required`,
		},
		{
			name:    "promoteFrom unknown env",
			yaml:    "version: 1\nenvironments:\n  prod: {policy: manual, promoteFrom: dev, gcp: {project: p, region: r}, services: d}\n",
			wantErr: "unknown environment",
		},
		{
			name:    "promoteFrom self",
			yaml:    "version: 1\nenvironments:\n  prod: {policy: manual, promoteFrom: prod, gcp: {project: p, region: r}, services: d}\n",
			wantErr: "must not point to itself",
		},
		{
			name:    "unknown field rejected",
			yaml:    "version: 1\nenvironments:\n  dev: {policy: manual, gcp: {project: p, region: r}, services: d, replicas: 3}\n",
			wantErr: "field replicas not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeRepo(t, map[string]string{ConfigFileName: tt.yaml})
			_, _, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadAllAutoWarns(t *testing.T) {
	root := writeRepo(t, map[string]string{
		ConfigFileName: "version: 1\nenvironments:\n  dev: {policy: auto, branch: main, gcp: {project: p, region: r}, services: d}\n",
	})
	_, warnings, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "auto") {
		t.Errorf("want all-auto warning, got %v", warnings)
	}
}

func TestServiceValidation(t *testing.T) {
	base := "version: 1\nenvironments:\n  dev: {policy: manual, gcp: {project: p, region: r}, services: svcs}\n"
	tests := []struct {
		name    string
		svc     string
		wantErr string
	}{
		{
			name:    "tag reference rejected",
			svc:     "name: app\nimage: gcr.io/p/app:latest\n",
			wantErr: "not digest-pinned",
		},
		{
			name:    "missing name",
			svc:     "image: gcr.io/p/app@" + digestA + "\n",
			wantErr: `"name" is required`,
		},
		{
			name:    "malformed digest",
			svc:     "name: app\nimage: gcr.io/p/app@sha256:tooshort\n",
			wantErr: "malformed digest",
		},
		{
			name:    "max below min",
			svc:     "name: app\nimage: gcr.io/p/app@" + digestA + "\nscaling: {min: 5, max: 2}\n",
			wantErr: "must be >=",
		},
		{
			name:    "env with both value and secret",
			svc:     "name: app\nimage: gcr.io/p/app@" + digestA + "\nenv: [{name: A, value: x, secret: s}]\n",
			wantErr: `sets both "value" and "secret"`,
		},
		{
			name:    "secret version without a secret",
			svc:     "name: app\nimage: gcr.io/p/app@" + digestA + "\nenv: [{name: A, value: x, version: \"3\"}]\n",
			wantErr: `sets "version" without "secret"`,
		},
		{
			name:    "duplicate env name",
			svc:     "name: app\nimage: gcr.io/p/app@" + digestA + "\nenv: [{name: A, value: x}, {name: A, value: y}]\n",
			wantErr: `env "A" is declared twice`,
		},
		{
			// The manifest name is free-form, but cloudRunName reaches the API.
			name:    "invalid cloudRunName",
			svc:     "name: app\ncloudRunName: App_Prod\nimage: gcr.io/p/app@" + digestA + "\n",
			wantErr: "not a valid Cloud Run service name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeRepo(t, map[string]string{
				ConfigFileName:  base,
				"svcs/app.yaml": tt.svc,
			})
			repo, _, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			env, _ := repo.Environment("dev")
			_, err = repo.LoadServices(env)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("LoadServices error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestFindRoot(t *testing.T) {
	root := writeRepo(t, map[string]string{
		ConfigFileName: "version: 1\nenvironments:\n  dev: {policy: manual, gcp: {project: p, region: r}, services: d}\n",
		"a/b/file.txt": "x",
	})
	got, err := FindRoot(filepath.Join(root, "a", "b"))
	if err != nil {
		t.Fatal(err)
	}
	// t.TempDir may contain symlinks on darwin; compare resolved paths.
	wantResolved, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Errorf("FindRoot = %q, want %q", got, root)
	}

	if _, err := FindRoot(t.TempDir()); err == nil {
		t.Error("FindRoot in empty dir: want error")
	}
}

func TestUpdateServiceImage(t *testing.T) {
	oldImage := "gcr.io/p/app@" + digestA
	newImage := "gcr.io/p/app@sha256:" + strings.Repeat("b", 64)
	root := writeRepo(t, map[string]string{
		ConfigFileName:  "version: 1\nenvironments:\n  dev: {policy: manual, gcp: {project: p, region: r}, services: svcs}\n",
		"svcs/app.yaml": "# keep this comment\nname: app\nimage: " + oldImage + "\n",
	})
	repo, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	env, _ := repo.Environment("dev")
	services, err := repo.LoadServices(env)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.UpdateServiceImage(services[0], newImage); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "svcs", "app.yaml"))
	if !strings.Contains(string(data), newImage) || !strings.Contains(string(data), "# keep this comment") {
		t.Errorf("file after update:\n%s", data)
	}
	if services[0].Image != newImage {
		t.Errorf("svc.Image not updated: %s", services[0].Image)
	}

	// A second replace of the old image must fail (no occurrence left).
	if err := repo.UpdateServiceImage(&Service{File: services[0].File, Image: oldImage}, newImage); err == nil {
		t.Error("replacing missing image: want error")
	}
}

func TestSplitDigest(t *testing.T) {
	path, digest, err := SplitDigest("gcr.io/p/app@" + digestA)
	if err != nil || path != "gcr.io/p/app" || digest != digestA {
		t.Errorf("SplitDigest = %q, %q, %v", path, digest, err)
	}
	if WithDigest(path, digest) != "gcr.io/p/app@"+digestA {
		t.Error("WithDigest roundtrip failed")
	}
	if ShortDigest(digestA) != "aaaaaaaaaaaa" {
		t.Errorf("ShortDigest = %q", ShortDigest(digestA))
	}
	for _, bad := range []string{"gcr.io/p/app:latest", "gcr.io/p/app", "@sha256:" + strings.Repeat("a", 64)} {
		if _, _, err := SplitDigest(bad); err == nil {
			t.Errorf("SplitDigest(%q): want error", bad)
		}
	}
}

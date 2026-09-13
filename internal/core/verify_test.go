package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Retr0413/wataridori/internal/cloudrun"
)

type verificationHTTP func(*http.Request) (*http.Response, error)

func (f verificationHTTP) Do(r *http.Request) (*http.Response, error) { return f(r) }

func TestVerifyDeploymentRequiresEveryGateAndNeverMutates(t *testing.T) {
	for _, failure := range []string{"", "digest", "ready", "traffic", "http", "registry", "missing"} {
		t.Run(failure, func(t *testing.T) {
			e := newTestEngine(t, false)
			image := "reg.example/dev/my-app@" + digestNew
			d := &cloudrun.Deployed{Image: image, Revision: "revision-1", Ready: true, TrafficPercent: 100, URL: "https://service.run.app"}
			e.cloudRun.deployed["dev/my-app"] = d
			switch failure {
			case "digest":
				d.Image = "reg.example/dev/my-app@" + digestOld
			case "ready":
				d.Ready = false
			case "traffic":
				d.TrafficPercent = 50
			case "missing":
				delete(e.cloudRun.deployed, "dev/my-app")
			case "registry":
				e.copier.verifyError = errors.New("not found")
			}
			e.HTTP = verificationHTTP(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != "https://service.run.app/ready" {
					t.Fatalf("unexpected URL %s", r.URL)
				}
				status := 200
				if failure == "http" {
					status = 503
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}, nil
			})
			r, err := e.VerifyDeployment(context.Background(), "dev", "my-app", image, []string{"/ready"})
			if err != nil {
				t.Fatal(err)
			}
			if r.Verified != (failure == "") {
				t.Fatalf("result=%+v", r)
			}
			if len(e.cloudRun.applied) > 0 || len(e.history.entries) > 0 || e.committer.commits > 0 {
				t.Fatal("verification mutated state")
			}
		})
	}
}

func TestVerifyRejectsChangedDesiredAndUntrustedHealthPaths(t *testing.T) {
	e := newTestEngine(t, false)
	for _, path := range []string{"//evil.example/x", "https://evil.example/x", "/health?token=x"} {
		_, err := e.VerifyDeployment(context.Background(), "dev", "my-app", "reg.example/dev/my-app@"+digestNew, []string{path})
		if err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
	_, err := e.VerifyDeployment(context.Background(), "dev", "my-app", "reg.example/dev/my-app@"+digestOld, nil)
	mustContain(t, err, "differs from Git")
}

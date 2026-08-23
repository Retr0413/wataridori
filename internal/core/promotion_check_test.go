package core

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Retr0413/wataridori/internal/cloudrun"
)

type fakeHTTPDoer struct {
	status int
	urls   []string
}

func (f *fakeHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	f.urls = append(f.urls, req.URL.String())
	return &http.Response{StatusCode: f.status, Body: io.NopCloser(strings.NewReader("secret body is ignored"))}, nil
}

func promotionProvenance() Provenance {
	return Provenance{
		SourceRepository: "Retr0413/BrickLog",
		SourceCommit:     "1234567890abcdef1234567890abcdef12345678",
		WorkflowRun:      "https://github.com/Retr0413/BrickLog/actions/runs/123",
	}
}

func TestCheckPromotionEligible(t *testing.T) {
	e := newTestEngine(t, false)
	e.cloudRun.deployed["dev/my-app"] = &cloudrun.Deployed{
		Service: "my-app", Image: "reg.example/dev/my-app@" + digestNew,
		Revision: "my-app-00002", Ready: true, TrafficPercent: 100,
		URL: "https://my-app.example.run.app",
	}
	httpDoer := &fakeHTTPDoer{status: http.StatusOK}
	e.HTTP = httpDoer
	res, err := e.CheckPromotion(context.Background(), PromotionCheckRequest{
		From: "dev", To: "prod", Service: "my-app", Provenance: promotionProvenance(),
		HTTPPaths: []string{"/health", "/ready"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Eligible || res.Noop || len(res.Items) != 1 || !res.Items[0].Eligible {
		t.Fatalf("evidence = %+v", res)
	}
	if len(httpDoer.urls) != 2 || httpDoer.urls[0] != "https://my-app.example.run.app/health" {
		t.Fatalf("HTTP URLs = %v", httpDoer.urls)
	}
}

func TestCheckPromotionRejectsDriftAndArbitraryHTTPHost(t *testing.T) {
	e := newTestEngine(t, false)
	e.cloudRun.deployed["dev/my-app"] = &cloudrun.Deployed{
		Service: "my-app", Image: "reg.example/dev/my-app@" + digestOld,
		Ready: true, TrafficPercent: 100, URL: "https://my-app.example.run.app",
	}
	e.HTTP = &fakeHTTPDoer{status: http.StatusOK}
	res, err := e.CheckPromotion(context.Background(), PromotionCheckRequest{
		From: "dev", To: "prod", Service: "my-app", Provenance: promotionProvenance(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Eligible || !strings.Contains(strings.Join(res.Reasons, " "), "digests differ") {
		t.Fatalf("evidence = %+v", res)
	}
	_, err = e.CheckPromotion(context.Background(), PromotionCheckRequest{
		From: "dev", To: "prod", Service: "my-app", Provenance: promotionProvenance(),
		HTTPPaths: []string{"https://evil.example/steal"},
	})
	mustContain(t, err, "relative absolute-path")
}

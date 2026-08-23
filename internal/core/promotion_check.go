package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Retr0413/wataridori/internal/manifest"
)

type PromotionCheckRequest struct {
	From        string        `json:"from,omitempty"`
	To          string        `json:"to"`
	Service     string        `json:"service,omitempty"`
	Provenance  Provenance    `json:"provenance"`
	HTTPPaths   []string      `json:"httpPaths,omitempty"`
	HTTPStatus  int           `json:"httpStatus,omitempty"`
	HTTPTimeout time.Duration `json:"-"`
	HTTPRetries int           `json:"httpRetries,omitempty"`
}

type PromotionCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

type PromotionHTTPCheck struct {
	Path       string `json:"path"`
	URL        string `json:"url"`
	StatusCode int    `json:"statusCode,omitempty"`
	Attempts   int    `json:"attempts"`
	Passed     bool   `json:"passed"`
	Error      string `json:"error,omitempty"`
}

type PromotionEvidenceItem struct {
	Service        string               `json:"service"`
	RunName        string               `json:"runName"`
	DesiredImage   string               `json:"desiredImage"`
	ActualImage    string               `json:"actualImage,omitempty"`
	TargetImage    string               `json:"targetImage"`
	Revision       string               `json:"revision,omitempty"`
	TrafficPercent int32                `json:"trafficPercent"`
	Checks         []PromotionCheck     `json:"checks"`
	HTTP           []PromotionHTTPCheck `json:"http,omitempty"`
	Eligible       bool                 `json:"eligible"`
	Reasons        []string             `json:"reasons,omitempty"`
}

type PromotionEvidence struct {
	EvidenceID string                  `json:"evidenceId"`
	CreatedAt  time.Time               `json:"createdAt"`
	From       string                  `json:"from"`
	To         string                  `json:"to"`
	Eligible   bool                    `json:"eligible"`
	Noop       bool                    `json:"noop"`
	Reasons    []string                `json:"reasons,omitempty"`
	Provenance Provenance              `json:"provenance"`
	Items      []PromotionEvidenceItem `json:"items"`
}

func (e *Engine) CheckPromotion(ctx context.Context, req PromotionCheckRequest) (*PromotionEvidence, error) {
	if err := req.Provenance.validate(); err != nil {
		return nil, err
	}
	if e.Verifier == nil || e.CloudRun == nil {
		return nil, fmt.Errorf("promotion inspection dependencies are not configured")
	}
	if len(req.HTTPPaths) > 0 && e.HTTP == nil {
		return nil, fmt.Errorf("HTTP checker is not configured")
	}
	if req.HTTPStatus == 0 {
		req.HTTPStatus = http.StatusOK
	}
	if req.HTTPStatus < 100 || req.HTTPStatus > 599 {
		return nil, fmt.Errorf("HTTP status must be between 100 and 599")
	}
	if req.HTTPTimeout <= 0 {
		req.HTTPTimeout = 5 * time.Second
	}
	if req.HTTPRetries <= 0 {
		req.HTTPRetries = 3
	}
	if req.HTTPRetries > 5 {
		return nil, fmt.Errorf("HTTP retries must not exceed 5")
	}
	for _, path := range req.HTTPPaths {
		if _, err := safeHealthURL("https://service.invalid", path); err != nil {
			return nil, err
		}
	}

	plan, err := e.PlanPromote(ctx, PromoteRequest{From: req.From, To: req.To, Service: req.Service})
	if err != nil {
		return nil, err
	}
	evidence := &PromotionEvidence{
		CreatedAt: time.Now().UTC(), From: plan.From, To: plan.To,
		Eligible: len(plan.Items) > 0, Noop: len(plan.Items) == 0, Provenance: req.Provenance,
	}
	if evidence.Noop {
		evidence.Reasons = append(evidence.Reasons, "target already has the source digest")
		evidence.EvidenceID = promotionEvidenceID(evidence)
		return evidence, nil
	}
	fromServices, err := e.Repo.LoadServices(plan.fromEnv)
	if err != nil {
		return nil, err
	}
	fromByName := make(map[string]*manifest.Service, len(fromServices))
	for _, svc := range fromServices {
		fromByName[svc.Name] = svc
	}
	toServices, err := e.services(plan.toEnv, req.Service)
	if err != nil {
		return nil, err
	}
	toByName := make(map[string]*manifest.Service, len(toServices))
	for _, svc := range toServices {
		toByName[svc.Name] = svc
	}
	for _, promotion := range plan.Items {
		target := toByName[promotion.Service]
		source := fromByName[promotion.Service]
		item := PromotionEvidenceItem{
			Service: source.Name, RunName: source.RunName(), DesiredImage: source.Image,
			TargetImage: target.Image, Eligible: true,
		}
		add := func(name string, passed bool, detail, reason string) {
			item.Checks = append(item.Checks, PromotionCheck{Name: name, Passed: passed, Detail: detail})
			if !passed {
				item.Eligible = false
				item.Reasons = append(item.Reasons, reason)
			}
		}
		actual, getErr := e.CloudRun.Get(ctx, plan.fromEnv, source.RunName())
		add("deployed", getErr == nil && actual != nil, "", "source service is not deployed")
		if getErr != nil {
			return nil, getErr
		}
		if actual != nil {
			item.ActualImage, item.Revision, item.TrafficPercent = actual.Image, actual.Revision, actual.TrafficPercent
			_, desiredDigest, _ := manifest.SplitDigest(source.Image)
			_, actualDigest, digestErr := manifest.SplitDigest(actual.Image)
			add("desired-actual-digest", digestErr == nil && desiredDigest == actualDigest, actualDigest, "source desired and serving digests differ")
			add("ready", actual.Ready, actual.ReadyMessage, "source revision is not Ready")
			add("traffic", actual.TrafficPercent == 100, strconv.Itoa(int(actual.TrafficPercent))+"%", "source revision is not serving 100% traffic")
			for _, path := range req.HTTPPaths {
				h := e.checkHTTP(ctx, actual.URL, path, req.HTTPStatus, req.HTTPTimeout, req.HTTPRetries)
				item.HTTP = append(item.HTTP, h)
				add("http "+path, h.Passed, strconv.Itoa(h.StatusCode), "HTTP check failed: "+path)
			}
		}
		verifyErr := e.Verifier.Verify(ctx, source.Image)
		add("registry-digest", verifyErr == nil, source.Image, errorDetail(verifyErr, "digest exists"))
		add("target-differs", true, target.Image, "target already has the source digest")
		add("no-production-registry-mutation", !promotion.NeedsCopy, "", "promotion requires a production registry copy")
		if !item.Eligible {
			evidence.Eligible = false
			evidence.Reasons = append(evidence.Reasons, source.Name+": "+strings.Join(item.Reasons, "; "))
		}
		evidence.Items = append(evidence.Items, item)
	}
	evidence.EvidenceID = promotionEvidenceID(evidence)
	return evidence, nil
}

func errorDetail(err error, success string) string {
	if err != nil {
		return err.Error()
	}
	return success
}

func safeHealthURL(base, path string) (string, error) {
	b, err := url.Parse(base)
	if err != nil || b.Scheme != "https" || b.Host == "" || b.User != nil {
		return "", fmt.Errorf("service URL from Cloud Run must be HTTPS")
	}
	p, err := url.Parse(path)
	if err != nil || !strings.HasPrefix(path, "/") || p.IsAbs() || p.Host != "" || p.User != nil || p.RawQuery != "" || p.Fragment != "" || strings.HasPrefix(path, "//") {
		return "", fmt.Errorf("HTTP path %q must be a relative absolute-path without host, query, or fragment", path)
	}
	b.Path, b.RawPath, b.RawQuery, b.Fragment = p.Path, p.RawPath, "", ""
	return b.String(), nil
}

func (e *Engine) checkHTTP(ctx context.Context, base, path string, expected int, timeout time.Duration, retries int) PromotionHTTPCheck {
	u, err := safeHealthURL(base, path)
	result := PromotionHTTPCheck{Path: path, URL: u}
	if err != nil {
		result.Error = err.Error()
		return result
	}
	for attempt := 1; attempt <= retries; attempt++ {
		result.Attempts = attempt
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		req, reqErr := http.NewRequestWithContext(checkCtx, http.MethodGet, u, nil)
		if reqErr != nil {
			cancel()
			result.Error = reqErr.Error()
			return result
		}
		resp, doErr := e.HTTP.Do(req)
		if doErr == nil {
			result.StatusCode = resp.StatusCode
			_ = resp.Body.Close()
			if resp.StatusCode == expected {
				cancel()
				result.Passed = true
				result.Error = ""
				return result
			}
			result.Error = fmt.Sprintf("expected HTTP %d, got %d", expected, resp.StatusCode)
		} else {
			result.Error = doErr.Error()
		}
		cancel()
	}
	return result
}

func promotionEvidenceID(e *PromotionEvidence) string {
	parts := []string{
		e.From, e.To, strings.ToLower(e.Provenance.SourceRepository),
		strings.ToLower(e.Provenance.SourceCommit), e.Provenance.WorkflowRun,
		strconv.FormatBool(e.Eligible), strconv.FormatBool(e.Noop), strings.Join(e.Reasons, "\n"),
	}
	for _, item := range e.Items {
		parts = append(parts, item.Service, item.DesiredImage, item.ActualImage, item.TargetImage,
			item.Revision, strconv.Itoa(int(item.TrafficPercent)), strconv.FormatBool(item.Eligible), strings.Join(item.Reasons, "\n"))
		for _, check := range item.Checks {
			parts = append(parts, check.Name, strconv.FormatBool(check.Passed), check.Detail)
		}
		for _, check := range item.HTTP {
			parts = append(parts, check.Path, check.URL, strconv.Itoa(check.StatusCode),
				strconv.Itoa(check.Attempts), strconv.FormatBool(check.Passed), check.Error)
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

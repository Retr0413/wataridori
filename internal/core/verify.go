package core

import (
	"context"
	"fmt"
	"time"

	"github.com/Retr0413/wataridori/internal/manifest"
)

// Verification is an observation, not an authorization or a deployment record.
type Verification struct {
	Environment    string               `json:"environment"`
	Service        string               `json:"service"`
	DesiredImage   string               `json:"desiredImage"`
	ActualImage    string               `json:"actualImage"`
	Revision       string               `json:"revision"`
	Ready          bool                 `json:"ready"`
	TrafficPercent int32                `json:"trafficPercent"`
	Verified       bool                 `json:"verified"`
	ObservedAt     time.Time            `json:"observedAt"`
	HTTP           []PromotionHTTPCheck `json:"http"`
	Reasons        []string             `json:"reasons"`
}

// VerifyDeployment checks the exact selected artifact even after promotion has
// made source and target manifests equal (where CheckPromotion is a no-op).
func (e *Engine) VerifyDeployment(ctx context.Context, envName, service, image string, paths []string) (*Verification, error) {
	r := &Verification{Environment: envName, Service: service, DesiredImage: image, ObservedAt: time.Now().UTC()}
	if _, _, err := manifest.SplitDigest(image); err != nil {
		return r, err
	}
	if service == "" || len(paths) > 10 {
		return r, fmt.Errorf("one service and at most 10 HTTP paths are required")
	}
	for _, path := range paths {
		if _, err := safeHealthURL("https://service.invalid", path); err != nil {
			return r, err
		}
	}
	env, err := e.Repo.Environment(envName)
	if err != nil {
		return r, err
	}
	services, err := e.services(env, service)
	if err != nil {
		return r, err
	}
	if services[0].Image != image {
		return r, fmt.Errorf("selected image differs from Git desired state")
	}
	actual, err := e.CloudRun.Get(ctx, env, services[0].RunName())
	if err != nil {
		return r, err
	}
	if actual == nil {
		r.Reasons = append(r.Reasons, "service is not deployed")
		return r, nil
	}
	r.ActualImage, r.Revision, r.Ready, r.TrafficPercent = actual.Image, actual.Revision, actual.Ready, actual.TrafficPercent
	_, desired, _ := manifest.SplitDigest(image)
	_, serving, digestErr := manifest.SplitDigest(actual.Image)
	if digestErr != nil || desired != serving {
		r.Reasons = append(r.Reasons, "serving digest differs from selected digest")
	}
	if !actual.Ready {
		r.Reasons = append(r.Reasons, "revision is not Ready")
	}
	if actual.TrafficPercent != 100 {
		r.Reasons = append(r.Reasons, "selected revision does not serve 100% traffic")
	}
	if err := e.Verifier.Verify(ctx, image); err != nil {
		r.Reasons = append(r.Reasons, "registry verification failed: "+err.Error())
	}
	for _, path := range paths {
		h := e.checkHTTP(ctx, actual.URL, path, 200, 5*time.Second, 3)
		r.HTTP = append(r.HTTP, h)
		if !h.Passed {
			r.Reasons = append(r.Reasons, "HTTP verification failed: "+path)
		}
	}
	r.Verified = len(r.Reasons) == 0
	return r, nil
}

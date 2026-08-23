package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/Retr0413/wataridori/internal/manifest"
)

// ImageEventRequest is the authenticated hand-off from application CI to Git
// desired state. Git ordering and compare-and-swap are enforced by the caller.
type ImageEventRequest struct {
	Env        string        `json:"env"`
	Service    string        `json:"service"`
	Image      string        `json:"image"`
	Provenance Provenance    `json:"provenance"`
	OccurredAt time.Time     `json:"occurredAt"`
	MaxAge     time.Duration `json:"-"`
}

type ImageEventResult struct {
	EventID    string     `json:"eventId"`
	Env        string     `json:"env"`
	Service    string     `json:"service"`
	File       string     `json:"file"`
	OldImage   string     `json:"oldImage"`
	Image      string     `json:"image"`
	Changed    bool       `json:"changed"`
	OccurredAt time.Time  `json:"occurredAt"`
	Provenance Provenance `json:"provenance"`
}

func (e *Engine) RecordImageEvent(ctx context.Context, req ImageEventRequest) (*ImageEventResult, error) {
	if err := req.Provenance.validate(); err != nil {
		return nil, err
	}
	if req.Service == "" {
		return nil, fmt.Errorf("service is required")
	}
	if _, _, err := manifest.SplitDigest(req.Image); err != nil {
		return nil, err
	}
	env, err := e.Repo.Environment(req.Env)
	if err != nil {
		return nil, err
	}
	if env.Policy != manifest.PolicyAuto {
		return nil, fmt.Errorf("environment %q uses policy %q; policy %q is required", env.Name, env.Policy, manifest.PolicyAuto)
	}
	if _, err := e.services(env, req.Service); err != nil {
		return nil, err
	}
	if req.OccurredAt.IsZero() {
		req.OccurredAt = time.Now().UTC()
	}
	if req.OccurredAt.After(time.Now().Add(5 * time.Minute)) {
		return nil, fmt.Errorf("artifact event occurred-at is in the future")
	}
	if req.MaxAge > 0 && time.Since(req.OccurredAt) > req.MaxAge {
		return nil, fmt.Errorf("artifact event expired: occurred at %s with max age %s", req.OccurredAt.Format(time.RFC3339), req.MaxAge)
	}
	if e.Verifier == nil {
		return nil, fmt.Errorf("image verifier is not configured")
	}
	if err := e.Verifier.Verify(ctx, req.Image); err != nil {
		return nil, err
	}
	set, err := e.SetManifestImage(SetManifestImageRequest{
		Env: req.Env, Service: req.Service, Image: req.Image, RequirePolicy: manifest.PolicyAuto,
	})
	if err != nil {
		return nil, err
	}
	return &ImageEventResult{
		EventID: eventID(req), Env: set.Env, Service: set.Service, File: set.File,
		OldImage: set.OldImage, Image: set.NewImage, Changed: set.Changed,
		OccurredAt: req.OccurredAt.UTC(), Provenance: req.Provenance,
	}, nil
}

func eventID(req ImageEventRequest) string {
	canonical := strings.Join([]string{
		req.Env, req.Service, req.Image, strings.ToLower(req.Provenance.SourceRepository),
		strings.ToLower(req.Provenance.SourceCommit), req.Provenance.WorkflowRun,
	}, "\x00")
	sum := sha256.Sum256([]byte(canonical))
	return "sha256:" + hex.EncodeToString(sum[:])
}

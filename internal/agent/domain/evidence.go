package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type TrustLevel string

const (
	TrustConfiguration TrustLevel = "trusted_configuration"
	TrustObservation   TrustLevel = "structured_observation"
	TrustUntrusted     TrustLevel = "untrusted_content"
)

func (t TrustLevel) Valid() bool {
	switch t {
	case TrustConfiguration, TrustObservation, TrustUntrusted:
		return true
	default:
		return false
	}
}

type EvidenceSource string

const (
	EvidenceDeploymentPlan   EvidenceSource = "deployment_plan"
	EvidenceDeploymentTrace  EvidenceSource = "deployment_trace"
	EvidenceCloudRun         EvidenceSource = "cloud_run"
	EvidenceArtifactRegistry EvidenceSource = "artifact_registry"
	EvidenceGitHub           EvidenceSource = "github"
	EvidenceGitDiff          EvidenceSource = "git_diff"
	EvidenceLog              EvidenceSource = "log"
	EvidenceRunbook          EvidenceSource = "runbook"
)

func (s EvidenceSource) Valid() bool {
	switch s {
	case EvidenceDeploymentPlan, EvidenceDeploymentTrace, EvidenceCloudRun,
		EvidenceArtifactRegistry, EvidenceGitHub, EvidenceGitDiff, EvidenceLog, EvidenceRunbook:
		return true
	default:
		return false
	}
}

// Evidence is normalized before it enters model context. Sanitized indicates
// that untrusted content passed through the configured redaction boundary even
// when no text ultimately needed replacement.
type Evidence struct {
	ID          string         `json:"id"`
	Source      EvidenceSource `json:"source"`
	Trust       TrustLevel     `json:"trust"`
	ObservedAt  time.Time      `json:"observedAt"`
	ContentHash string         `json:"contentHash"`
	Content     string         `json:"content"`
	Sanitized   bool           `json:"sanitized"`
	Truncated   bool           `json:"truncated"`
}

func NewEvidence(id string, source EvidenceSource, trust TrustLevel, observedAt time.Time, content string, sanitized, truncated bool) (Evidence, error) {
	sum := sha256.Sum256([]byte(content))
	e := Evidence{
		ID:          id,
		Source:      source,
		Trust:       trust,
		ObservedAt:  observedAt.UTC(),
		ContentHash: hex.EncodeToString(sum[:]),
		Content:     content,
		Sanitized:   sanitized,
		Truncated:   truncated,
	}
	if err := e.Validate(); err != nil {
		return Evidence{}, err
	}
	return e, nil
}

func (e Evidence) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return errors.New("agent domain: evidence ID is required")
	}
	if !e.Source.Valid() || !e.Trust.Valid() {
		return errors.New("agent domain: evidence source and trust are required")
	}
	if e.ObservedAt.IsZero() || strings.TrimSpace(e.ContentHash) == "" {
		return errors.New("agent domain: evidence observation metadata is required")
	}
	if e.Trust == TrustUntrusted && !e.Sanitized {
		return errors.New("agent domain: untrusted evidence must cross the sanitization boundary")
	}
	return nil
}

package domain

import (
	"errors"
	"strings"
	"time"
)

// Action is a recovery operation that the agent may propose. The policy layer
// must authorize a proposal before any action reaches an executor.
type Action string

const (
	ActionWait       Action = "wait_and_reobserve"
	ActionRetry      Action = "retry_same_plan"
	ActionRollback   Action = "rollback_last_verified"
	ActionQuarantine Action = "quarantine_digest"
	ActionAbort      Action = "abort_recovery"
	ActionEscalate   Action = "escalate"
)

func (a Action) Valid() bool {
	switch a {
	case ActionWait, ActionRetry, ActionRollback, ActionQuarantine, ActionAbort, ActionEscalate:
		return true
	default:
		return false
	}
}

// Confidence deliberately uses coarse values. A numeric model score is not an
// authorization signal and must not be interpreted as one.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

func (c Confidence) Valid() bool {
	switch c {
	case ConfidenceLow, ConfidenceMedium, ConfidenceHigh:
		return true
	default:
		return false
	}
}

// Proposal is the structured, non-executable result of model reasoning.
type Proposal struct {
	Action          Action     `json:"action"`
	FailureCategory string     `json:"failureCategory"`
	Confidence      Confidence `json:"confidence"`
	EvidenceIDs     []string   `json:"evidenceIds"`
	TargetRevision  string     `json:"targetRevision,omitempty"`
	TargetDigest    string     `json:"targetDigest,omitempty"`
	Reason          string     `json:"reason"`
	MissingEvidence []string   `json:"missingEvidence,omitempty"`
	Contradictions  []string   `json:"contradictions,omitempty"`
}

func (p Proposal) Validate() error {
	if !p.Action.Valid() {
		return errors.New("agent domain: invalid recovery action")
	}
	if !p.Confidence.Valid() {
		return errors.New("agent domain: invalid confidence")
	}
	if strings.TrimSpace(p.Reason) == "" {
		return errors.New("agent domain: proposal reason is required")
	}
	if len(p.EvidenceIDs) == 0 {
		return errors.New("agent domain: proposal requires evidence")
	}
	if p.Action == ActionRollback {
		if strings.TrimSpace(p.TargetRevision) == "" || strings.TrimSpace(p.TargetDigest) == "" {
			return errors.New("agent domain: rollback proposal requires target revision and digest")
		}
	}
	return nil
}

// AuthorizedPlan is the only recovery value that an executor may accept. It
// is produced by deterministic policy code, never directly by a model adapter.
type AuthorizedPlan struct {
	ID               string    `json:"id"`
	RecoveryRunID    string    `json:"recoveryRunId"`
	Fingerprint      string    `json:"fingerprint"`
	Action           Action    `json:"action"`
	Environment      string    `json:"environment"`
	Service          string    `json:"service"`
	ExpectedRevision string    `json:"expectedRevision,omitempty"`
	TargetRevision   string    `json:"targetRevision,omitempty"`
	TargetDigest     string    `json:"targetDigest,omitempty"`
	PolicyCommit     string    `json:"policyCommit"`
	AuthorizedAt     time.Time `json:"authorizedAt"`
}

func (p AuthorizedPlan) Validate() error {
	if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.RecoveryRunID) == "" {
		return errors.New("agent domain: authorized plan identity is required")
	}
	if strings.TrimSpace(p.Fingerprint) == "" {
		return errors.New("agent domain: authorized plan fingerprint is required")
	}
	if !p.Action.Valid() {
		return errors.New("agent domain: authorized plan action is invalid")
	}
	if strings.TrimSpace(p.Environment) == "" || strings.TrimSpace(p.Service) == "" {
		return errors.New("agent domain: authorized plan scope is required")
	}
	if strings.TrimSpace(p.PolicyCommit) == "" || p.AuthorizedAt.IsZero() {
		return errors.New("agent domain: authorized plan policy evidence is required")
	}
	if p.Action == ActionRollback {
		if strings.TrimSpace(p.TargetRevision) == "" || strings.TrimSpace(p.TargetDigest) == "" {
			return errors.New("agent domain: authorized rollback requires target revision and digest")
		}
	}
	return nil
}

package domain

import (
	"errors"
	"strings"
	"time"
)

type RecoveryMode string

const (
	RecoveryDisabled   RecoveryMode = "disabled"
	RecoveryObserve    RecoveryMode = "observe"
	RecoveryAutonomous RecoveryMode = "autonomous"
)

func (m RecoveryMode) Valid() bool {
	switch m {
	case RecoveryDisabled, RecoveryObserve, RecoveryAutonomous:
		return true
	default:
		return false
	}
}

// Budget bounds one model-assisted investigation. Infrastructure adapters may
// impose stricter provider limits but may not expand these values.
type Budget struct {
	MaxSteps         int           `json:"maxSteps"`
	MaxToolCalls     int           `json:"maxToolCalls"`
	MaxEvidenceBytes int           `json:"maxEvidenceBytes"`
	MaxDuration      time.Duration `json:"maxDuration"`
}

func (b Budget) Validate() error {
	if b.MaxSteps <= 0 || b.MaxToolCalls < 0 || b.MaxEvidenceBytes <= 0 || b.MaxDuration <= 0 {
		return errors.New("agent domain: recovery budget must contain positive bounds")
	}
	return nil
}

// PolicySnapshot is loaded from a protected Git commit before model reasoning
// starts. It is immutable for the lifetime of a recovery run.
type PolicySnapshot struct {
	Version        int          `json:"version"`
	Commit         string       `json:"commit"`
	Mode           RecoveryMode `json:"mode"`
	AllowedActions []Action     `json:"allowedActions"`
	Budget         Budget       `json:"budget"`
}

func (p PolicySnapshot) Validate() error {
	if p.Version <= 0 || strings.TrimSpace(p.Commit) == "" {
		return errors.New("agent domain: recovery policy version and commit are required")
	}
	if !p.Mode.Valid() {
		return errors.New("agent domain: invalid recovery mode")
	}
	if err := p.Budget.Validate(); err != nil {
		return err
	}
	seen := make(map[Action]struct{}, len(p.AllowedActions))
	for _, action := range p.AllowedActions {
		if !action.Valid() {
			return errors.New("agent domain: policy contains an invalid action")
		}
		if _, ok := seen[action]; ok {
			return errors.New("agent domain: policy contains a duplicate action")
		}
		seen[action] = struct{}{}
	}
	return nil
}

func (p PolicySnapshot) Allows(action Action) bool {
	if p.Mode != RecoveryAutonomous {
		return false
	}
	for _, allowed := range p.AllowedActions {
		if allowed == action {
			return true
		}
	}
	return false
}

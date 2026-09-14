package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Scope pins the deployment failure that one recovery run investigates.
type Scope struct {
	Repository      string    `json:"repository"`
	Environment     string    `json:"environment"`
	Service         string    `json:"service"`
	Digest          string    `json:"digest"`
	Revision        string    `json:"revision,omitempty"`
	WorkflowRun     string    `json:"workflowRun,omitempty"`
	OperationID     string    `json:"operationId,omitempty"`
	PlanFingerprint string    `json:"planFingerprint"`
	TraceID         string    `json:"traceId"`
	FailureStage    string    `json:"failureStage"`
	ObservedAt      time.Time `json:"observedAt"`
}

func (s Scope) Validate() error {
	required := []string{
		s.Repository,
		s.Environment,
		s.Service,
		s.Digest,
		s.PlanFingerprint,
		s.TraceID,
		s.FailureStage,
	}
	for _, value := range required {
		if strings.TrimSpace(value) == "" {
			return errors.New("agent domain: recovery scope is incomplete")
		}
	}
	if s.ObservedAt.IsZero() {
		return errors.New("agent domain: recovery scope observation time is required")
	}
	return nil
}

// RecoveryRun is the durable state-machine aggregate for one failure scope.
type RecoveryRun struct {
	ID        string         `json:"id"`
	Scope     Scope          `json:"scope"`
	Policy    PolicySnapshot `json:"policy"`
	State     State          `json:"state"`
	Version   uint64         `json:"version"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

func NewRecoveryRun(id string, scope Scope, policy PolicySnapshot, now time.Time) (*RecoveryRun, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("agent domain: recovery run ID is required")
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if now.IsZero() {
		return nil, errors.New("agent domain: recovery run creation time is required")
	}
	now = now.UTC()
	return &RecoveryRun{
		ID:        id,
		Scope:     scope,
		Policy:    policy,
		State:     StateDetected,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (r RecoveryRun) Validate() error {
	if strings.TrimSpace(r.ID) == "" || !r.State.Valid() || r.Version == 0 {
		return errors.New("agent domain: recovery run identity, state, and version are required")
	}
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	if r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() {
		return errors.New("agent domain: recovery run timestamps are required")
	}
	return nil
}

func (r *RecoveryRun) Transition(next State, now time.Time) error {
	if r == nil {
		return errors.New("agent domain: nil recovery run")
	}
	if !CanTransition(r.State, next) {
		return fmt.Errorf("agent domain: invalid recovery transition %q -> %q", r.State, next)
	}
	if now.IsZero() || now.Before(r.UpdatedAt) {
		return errors.New("agent domain: transition time must not precede the current state")
	}
	r.State = next
	r.Version++
	r.UpdatedAt = now.UTC()
	return nil
}

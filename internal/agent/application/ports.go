package application

import (
	"context"
	"time"

	"github.com/Retr0413/wataridori/internal/agent/domain"
)

// RecoveryUseCase is the inbound application boundary used by presentation
// adapters. A concrete orchestrator will implement this interface.
type RecoveryUseCase interface {
	Run(context.Context, RunCommand) (*RunResult, error)
}

type RunCommand struct {
	RecoveryRunID string
	Scope         domain.Scope
	Policy        domain.PolicySnapshot
}

type RunResult struct {
	Run domain.RecoveryRun
}

// Advisor is the only model-facing port. Its result is a non-executable
// proposal that still requires deterministic authorization.
type Advisor interface {
	Propose(context.Context, AgentInput) (domain.Proposal, error)
}

type EvidenceRequest struct {
	Scope     domain.Scope
	Sources   []domain.EvidenceSource
	MaxItems  int
	MaxBytes  int
	StartedAt time.Time
}

type EvidenceCollector interface {
	Collect(context.Context, EvidenceRequest) ([]domain.Evidence, error)
}

type PolicyAuthorizer interface {
	Authorize(context.Context, domain.RecoveryRun, domain.Proposal) (*domain.AuthorizedPlan, error)
}

type ExecutionResult struct {
	PlanID      string
	OperationID string
	StartedAt   time.Time
	FinishedAt  time.Time
}

type RecoveryExecutor interface {
	Execute(context.Context, domain.AuthorizedPlan) (*ExecutionResult, error)
}

type VerificationResult struct {
	Recovered      bool
	DesiredDigest  string
	ActualDigest   string
	Revision       string
	Ready          bool
	TrafficPercent int32
	ObservedAt     time.Time
	Reasons        []string
}

type RecoveryVerifier interface {
	Verify(context.Context, domain.RecoveryRun, domain.AuthorizedPlan, ExecutionResult) (*VerificationResult, error)
}

// RecoveryRepository persists state with optimistic concurrency. expectedVersion
// prevents duplicate or out-of-order workers from overwriting a newer state.
type RecoveryRepository interface {
	Create(context.Context, domain.RecoveryRun) error
	Get(context.Context, string) (*domain.RecoveryRun, error)
	Update(context.Context, domain.RecoveryRun, uint64) error
}

type Lease struct {
	RecoveryRunID string
	Owner         string
	ExpiresAt     time.Time
}

type LeaseManager interface {
	Acquire(context.Context, Lease) error
	Renew(context.Context, Lease) error
	Release(context.Context, Lease) error
}

type Clock interface {
	Now() time.Time
}

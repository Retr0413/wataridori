package infrastructure

import (
	"context"
	"errors"
	"time"

	"github.com/Retr0413/wataridori/internal/agent/application"
	"github.com/Retr0413/wataridori/internal/agent/domain"
)

var ErrModelNotConfigured = errors.New("agent infrastructure: model provider is not configured")

// UTCClock is the production clock adapter. Tests should provide a fixed clock
// through the application Clock port.
type UTCClock struct{}

func (UTCClock) Now() time.Time { return time.Now().UTC() }

// DisabledAdvisor is the fail-closed default used until a model provider is
// explicitly configured. It performs no network access and proposes no action.
type DisabledAdvisor struct{}

func (DisabledAdvisor) Propose(context.Context, application.AgentInput) (domain.Proposal, error) {
	return domain.Proposal{}, ErrModelNotConfigured
}

var _ application.Clock = UTCClock{}
var _ application.Advisor = DisabledAdvisor{}

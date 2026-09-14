package presentation

import (
	"context"
	"errors"

	"github.com/Retr0413/wataridori/internal/agent/application"
)

// Handler is a transport-neutral presentation adapter. CLI, workflow,
// controller, and Connect adapters may wrap it without duplicating recovery
// orchestration or model-context assembly.
type Handler struct {
	useCase application.RecoveryUseCase
}

func NewHandler(useCase application.RecoveryUseCase) (*Handler, error) {
	if useCase == nil {
		return nil, errors.New("agent presentation: recovery use case is required")
	}
	return &Handler{useCase: useCase}, nil
}

func (h *Handler) Run(ctx context.Context, command application.RunCommand) (*application.RunResult, error) {
	if h == nil || h.useCase == nil {
		return nil, errors.New("agent presentation: handler is not configured")
	}
	return h.useCase.Run(ctx, command)
}

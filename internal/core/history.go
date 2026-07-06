package core

import (
	"context"

	"github.com/Retr0413/wataridori/internal/store"
)

// HistoryRequest lists recorded operations, newest first.
// See docs/spec/phase1-cli.md §2.5.
type HistoryRequest struct {
	Env   string `json:"env,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type HistoryResult struct {
	Entries []store.Entry `json:"entries"`
}

func (e *Engine) ListHistory(ctx context.Context, req HistoryRequest) (*HistoryResult, error) {
	entries, err := e.History.List(ctx, store.ListOptions{Env: req.Env, Limit: req.Limit})
	if err != nil {
		return nil, err
	}
	return &HistoryResult{Entries: entries}, nil
}

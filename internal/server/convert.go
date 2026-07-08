package server

import (
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/Retr0413/wataridori/gen/wataridori/v1"
	"github.com/Retr0413/wataridori/internal/core"
	"github.com/Retr0413/wataridori/internal/store"
)

// rpcError maps a core error to a Connect status code. Validation-shaped
// errors become InvalidArgument; everything else is Internal. A code the
// caller already set (a *connect.Error) is passed through unchanged.
func rpcError(err error) error {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return err
	}
	var unknownSvc *core.UnknownServiceError
	if errors.As(err, &unknownSvc) {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}

// --- Status ---

func statusToProto(res *core.StatusResult) *v1.StatusResponse {
	out := &v1.StatusResponse{Drift: res.Drift}
	for _, s := range res.Services {
		out.Services = append(out.Services, &v1.ServiceStatus{
			Env:            s.Env,
			Service:        s.Service,
			DesiredImage:   s.DesiredImage,
			DesiredDigest:  s.DesiredDigest,
			ActualImage:    s.ActualImage,
			ActualDigest:   s.ActualDigest,
			Revision:       s.Revision,
			State:          syncStateToProto(s.State),
			Ready:          s.Ready,
			ReadyMessage:   s.ReadyMessage,
			TrafficPercent: s.TrafficPct,
			Url:            s.URL,
			ConsoleUrl:     s.ConsoleURL,
		})
	}
	return out
}

func syncStateToProto(s core.SyncState) v1.SyncState {
	switch s {
	case core.StateInSync:
		return v1.SyncState_SYNC_STATE_IN_SYNC
	case core.StateDrift:
		return v1.SyncState_SYNC_STATE_DRIFT
	case core.StateNotDeployed:
		return v1.SyncState_SYNC_STATE_NOT_DEPLOYED
	default:
		return v1.SyncState_SYNC_STATE_UNSPECIFIED
	}
}

// --- Apply ---

func applyFromProto(msg *v1.ApplyRequest) core.ApplyRequest {
	timeout := time.Duration(msg.GetTimeoutSeconds()) * time.Second
	if timeout <= 0 {
		timeout = core.DefaultApplyTimeout
	}
	return core.ApplyRequest{
		Env:     msg.GetEnv(),
		Service: msg.GetService(),
		DryRun:  msg.GetDryRun(),
		Timeout: timeout,
	}
}

func applyToProto(res *core.ApplyResult) *v1.ApplyResponse {
	out := &v1.ApplyResponse{Env: res.Env, DryRun: res.DryRun}
	for _, s := range res.Services {
		out.Services = append(out.Services, &v1.ApplyServiceResult{
			Service:      s.Service,
			DesiredImage: s.DesiredImage,
			ActualImage:  s.ActualImage,
			Revision:     s.Revision,
			Url:          s.URL,
			InSync:       s.InSync,
		})
	}
	return out
}

// --- Promote ---

func promoteItemsToProto(items []core.PromoteItem) []*v1.PromoteItem {
	var out []*v1.PromoteItem
	for _, it := range items {
		out = append(out, &v1.PromoteItem{
			Service:   it.Service,
			FromImage: it.FromImage,
			OldImage:  it.OldImage,
			NewImage:  it.NewImage,
			NeedsCopy: it.NeedsCopy,
		})
	}
	return out
}

func promotePlanToProto(p *core.PromotePlan) *v1.PlanPromoteResponse {
	return &v1.PlanPromoteResponse{From: p.From, To: p.To, Items: promoteItemsToProto(p.Items)}
}

func promoteResultToProto(r *core.PromoteResult) *v1.ExecutePromoteResponse {
	return &v1.ExecutePromoteResponse{
		From:     r.From,
		To:       r.To,
		CommitId: r.CommitID,
		Items:    promoteItemsToProto(r.Items),
	}
}

// --- Rollback ---

func rollbackItemsToProto(items []core.RollbackItem) []*v1.RollbackItem {
	var out []*v1.RollbackItem
	for _, it := range items {
		out = append(out, &v1.RollbackItem{
			Service:         it.Service,
			CurrentRevision: it.CurrentRevision,
			CurrentImage:    it.CurrentImage,
			TargetRevision:  it.TargetRevision,
			TargetImage:     it.TargetImage,
		})
	}
	return out
}

func rollbackPlanToProto(p *core.RollbackPlan) *v1.PlanRollbackResponse {
	return &v1.PlanRollbackResponse{Env: p.Env, Items: rollbackItemsToProto(p.Items)}
}

func rollbackResultToProto(r *core.RollbackResult) *v1.ExecuteRollbackResponse {
	return &v1.ExecuteRollbackResponse{Env: r.Env, Items: rollbackItemsToProto(r.Items)}
}

// --- History ---

func historyToProto(res *core.HistoryResult) *v1.HistoryResponse {
	out := &v1.HistoryResponse{}
	for _, e := range res.Entries {
		out.Entries = append(out.Entries, &v1.HistoryEntry{
			Id:      e.ID,
			Time:    timestamppb.New(e.Time),
			Actor:   e.Actor,
			Action:  actionToProto(e.Action),
			Env:     e.Env,
			Service: e.Service,
			Digest:  e.Digest,
			Detail:  e.Detail,
		})
	}
	return out
}

func actionToProto(a store.Action) v1.Action {
	switch a {
	case store.ActionApply:
		return v1.Action_ACTION_APPLY
	case store.ActionPromote:
		return v1.Action_ACTION_PROMOTE
	case store.ActionRollback:
		return v1.Action_ACTION_ROLLBACK
	default:
		return v1.Action_ACTION_UNSPECIFIED
	}
}

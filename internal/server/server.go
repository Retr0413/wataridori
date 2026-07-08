package server

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	v1 "github.com/Retr0413/wataridori/gen/wataridori/v1"
	"github.com/Retr0413/wataridori/gen/wataridori/v1/wataridoriv1connect"
	"github.com/Retr0413/wataridori/internal/core"
)

// UseCases is the slice of core.Engine the RPC handlers depend on.
// *core.Engine satisfies it; tests inject a fake.
type UseCases interface {
	Status(context.Context, core.StatusRequest) (*core.StatusResult, error)
	Apply(context.Context, core.ApplyRequest) (*core.ApplyResult, error)
	PlanPromote(context.Context, core.PromoteRequest) (*core.PromotePlan, error)
	ExecutePromote(context.Context, *core.PromotePlan) (*core.PromoteResult, error)
	PlanRollback(context.Context, core.RollbackRequest) (*core.RollbackPlan, error)
	ExecuteRollback(context.Context, *core.RollbackPlan) (*core.RollbackResult, error)
	ListHistory(context.Context, core.HistoryRequest) (*core.HistoryResult, error)
}

// EngineFunc builds the use cases for one request. The returned cleanup is
// always called when the request finishes (even on error). Loading state per
// request keeps the server in step with the manifest repo and Cloud Run.
type EngineFunc func(ctx context.Context) (UseCases, func(), error)

// Server implements wataridoriv1connect.DeploymentServiceHandler.
type Server struct {
	engine EngineFunc
}

// New returns a Server that builds its use cases via engine.
func New(engine EngineFunc) *Server {
	return &Server{engine: engine}
}

// Handler returns the route prefix and HTTP handler to mount. Callers wrap it
// with h2c to accept gRPC over cleartext HTTP/2.
func (s *Server) Handler(opts ...connect.HandlerOption) (string, http.Handler) {
	return wataridoriv1connect.NewDeploymentServiceHandler(s, opts...)
}

// use builds the engine and defers cleanup; handlers call it once at entry.
func (s *Server) use(ctx context.Context) (UseCases, func(), error) {
	uc, cleanup, err := s.engine(ctx)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, func() {}, connect.NewError(connect.CodeInternal, err)
	}
	return uc, cleanup, nil
}

func (s *Server) Status(ctx context.Context, req *connect.Request[v1.StatusRequest]) (*connect.Response[v1.StatusResponse], error) {
	uc, cleanup, err := s.use(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	res, err := uc.Status(ctx, core.StatusRequest{Env: req.Msg.GetEnv()})
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(statusToProto(res)), nil
}

func (s *Server) Apply(ctx context.Context, req *connect.Request[v1.ApplyRequest]) (*connect.Response[v1.ApplyResponse], error) {
	uc, cleanup, err := s.use(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	res, err := uc.Apply(ctx, applyFromProto(req.Msg))
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(applyToProto(res)), nil
}

func (s *Server) PlanPromote(ctx context.Context, req *connect.Request[v1.PlanPromoteRequest]) (*connect.Response[v1.PlanPromoteResponse], error) {
	uc, cleanup, err := s.use(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	plan, err := uc.PlanPromote(ctx, core.PromoteRequest{
		From: req.Msg.GetFrom(), To: req.Msg.GetTo(), Service: req.Msg.GetService(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(promotePlanToProto(plan)), nil
}

func (s *Server) ExecutePromote(ctx context.Context, req *connect.Request[v1.ExecutePromoteRequest]) (*connect.Response[v1.ExecutePromoteResponse], error) {
	uc, cleanup, err := s.use(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// Re-derive the plan from the request, then execute: the plan carries
	// unexported environment state, so it cannot cross the wire. Planning and
	// executing on the same engine keeps the call stateless.
	plan, err := uc.PlanPromote(ctx, core.PromoteRequest{
		From: req.Msg.GetFrom(), To: req.Msg.GetTo(), Service: req.Msg.GetService(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	res, err := uc.ExecutePromote(ctx, plan)
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(promoteResultToProto(res)), nil
}

func (s *Server) PlanRollback(ctx context.Context, req *connect.Request[v1.PlanRollbackRequest]) (*connect.Response[v1.PlanRollbackResponse], error) {
	uc, cleanup, err := s.use(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	plan, err := uc.PlanRollback(ctx, core.RollbackRequest{
		Env: req.Msg.GetEnv(), Service: req.Msg.GetService(), Revision: req.Msg.GetRevision(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(rollbackPlanToProto(plan)), nil
}

func (s *Server) ExecuteRollback(ctx context.Context, req *connect.Request[v1.ExecuteRollbackRequest]) (*connect.Response[v1.ExecuteRollbackResponse], error) {
	uc, cleanup, err := s.use(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	plan, err := uc.PlanRollback(ctx, core.RollbackRequest{
		Env: req.Msg.GetEnv(), Service: req.Msg.GetService(), Revision: req.Msg.GetRevision(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	res, err := uc.ExecuteRollback(ctx, plan)
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(rollbackResultToProto(res)), nil
}

func (s *Server) History(ctx context.Context, req *connect.Request[v1.HistoryRequest]) (*connect.Response[v1.HistoryResponse], error) {
	uc, cleanup, err := s.use(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	res, err := uc.ListHistory(ctx, core.HistoryRequest{
		Env: req.Msg.GetEnv(), Limit: int(req.Msg.GetLimit()),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(historyToProto(res)), nil
}

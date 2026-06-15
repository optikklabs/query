package nodes

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type NodeHandler struct {
	Service *Service
}

func (h *NodeHandler) GetInfrastructureNodes(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query node health", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetInfrastructureNodes(ctx, teamID, startMs, endMs)
	})
}

func (h *NodeHandler) GetInfrastructureNodeSummary(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query node summary", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetInfrastructureNodeSummary(ctx, teamID, startMs, endMs)
	})
}

func (h *NodeHandler) GetInfrastructureNodeServices(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	modulecommon.HandleRangeQuery(w, r, "Failed to query node services", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetInfrastructureNodeServices(ctx, teamID, host, startMs, endMs)
	})
}

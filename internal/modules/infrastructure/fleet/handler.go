package fleet

import (
	"context"
	"net/http"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func (h *Handler) GetFleetPods(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query fleet pods", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetFleetPods(ctx, teamID, startMs, endMs)
	})
}

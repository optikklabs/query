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
	host := r.URL.Query().Get("host")
	modulecommon.HandleRangeQuery(w, r, "Failed to query fleet pods", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetFleetPods(ctx, tenantID, startMs, endMs, host)
	})
}

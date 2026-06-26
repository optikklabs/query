package topology

import (
	"context"
	"net/http"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

// Handler serves the runtime service topology API.
type Handler struct {
	Service *Service
}

func (h *Handler) GetTopology(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	modulecommon.HandleRangeQuery(w, r, "Failed to build service topology", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopology(ctx, teamID, startMs, endMs, service)
	})
}

package topology

import (
	"context"
	"net/http"

	"github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func (h *Handler) GetTopology(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	httputil.HandleRangeQuery(w, r, "Failed to build service topology", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopology(ctx, tenantID, startMs, endMs, service)
	})
}

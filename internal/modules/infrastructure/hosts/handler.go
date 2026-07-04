package hosts

import (
	"context"
	"net/http"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

// GetHosts retrieves the host saturation list, optionally filtered by service.
func (h *Handler) GetHosts(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query hosts", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetHosts(ctx, tenantID, startMs, endMs, r.URL.Query().Get("service"))
	})
}

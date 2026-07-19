package explorer

import (
	"context"
	"net/http"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func (h *Handler) GetDatastoreSystems(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query datastore systems", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetDatastoreSystems(ctx, tenantID, startMs, endMs)
	})
}

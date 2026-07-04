package slowqueries

import (
	"context"
	"net/http"

	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func (h *Handler) GetSlowQueryPatterns(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query slow query patterns", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetSlowQueryPatterns(ctx, tenantID, startMs, endMs, filter.ParseFilters(r), filter.ParseLimit(r, 20))
	})
}

package latency

import (
	"context"
	"net/http"

	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func (h *Handler) GetLatencyBySystem(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query latency by system", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetLatencyBySystem(ctx, teamID, startMs, endMs, filter.ParseFilters(r))
	})
}

package volume

import (
	"context"
	"net/http"

	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func (h *Handler) GetOpsBySystem(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query ops by system", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetOpsBySystem(ctx, teamID, startMs, endMs, filter.ParseFilters(r))
	})
}

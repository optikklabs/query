package producer

import (
	"context"
	"net/http"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func (h *Handler) GetProduceRateByTopic(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query produce rate by topic", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetProduceRateByTopic(ctx, teamID, startMs, endMs)
	})
}

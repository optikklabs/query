package consumer

import (
	"context"
	"net/http"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func (h *Handler) GetConsumeRateByTopic(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query consume rate by topic", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetConsumeRateByTopic(ctx, teamID, startMs, endMs)
	})
}

func (h *Handler) GetConsumerLagByGroup(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query consumer lag by group", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetConsumerLagByGroup(ctx, teamID, startMs, endMs)
	})
}

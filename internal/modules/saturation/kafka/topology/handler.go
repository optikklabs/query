package topology

import (
	"context"
	"net/http"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

// GetTopology returns the Kafka producers->topics->consumers graph.
func (h *Handler) GetTopology(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to build kafka topology", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopology(ctx, teamID, startMs, endMs, "")
	})
}

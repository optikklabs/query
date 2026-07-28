package ingestion

import (
	"context"
	"net/http"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query ingestion overview", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.svc.Overview(ctx, tenantID, startMs, endMs)
	})
}

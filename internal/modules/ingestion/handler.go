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

// Summary powers GET /api/v1/ingestion/summary — KPI strip + by-type breakdown.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query ingestion summary", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.svc.Summary(ctx, tenantID, startMs, endMs)
	})
}

// Cost powers GET /api/v1/ingestion/cost — the tenant's estimated bill.
func (h *Handler) Cost(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query ingestion cost", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.svc.Cost(ctx, tenantID, startMs, endMs)
	})
}

// Timeseries powers GET /api/v1/ingestion/timeseries?groupBy=type|service.
func (h *Handler) Timeseries(w http.ResponseWriter, r *http.Request) {
	groupBy := r.URL.Query().Get("groupBy")
	modulecommon.HandleRangeQuery(w, r, "Failed to query ingestion timeseries", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.svc.Timeseries(ctx, tenantID, startMs, endMs, groupBy)
	})
}

// Services powers GET /api/v1/ingestion/services — top ingesting services table.
func (h *Handler) Services(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query ingestion services", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.svc.Services(ctx, tenantID, startMs, endMs)
	})
}

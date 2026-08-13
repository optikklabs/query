package redfleet

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/modules/services/redfleet/filter"
	"github.com/optikklabs/query/internal/modules/services/redfleet/models"
	"github.com/optikklabs/query/internal/modules/services/redfleet/service"
	"github.com/optikklabs/query/internal/shared/errorcode"

	"github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *service.Service
}

func parseFilters(r *http.Request, tenantID, startMs, endMs int64) filter.Filters {
	f := filter.Filters{TenantID: tenantID, StartMs: startMs, EndMs: endMs}
	if sn := r.URL.Query().Get("serviceName"); sn != "" {
		f.Services = []string{sn}
	}
	if ss := r.URL.Query()["services"]; len(ss) > 0 {
		f.Services = ss
	}
	if sn := chi.URLParam(r, "serviceName"); sn != "" {
		f.Services = []string{sn}
	}
	return f
}

func (h *Handler) GetFleetOverview(w http.ResponseWriter, r *http.Request) {
	httputil.HandleComparableRangeQuery(w, r, "Failed to query fleet overview", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetFleetOverview(ctx, parseFilters(r, tenantID, startMs, endMs))
	})
}

func (h *Handler) GetRequestAndErrorRateTimeSeries(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query request and error rate time series", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetRequestAndErrorRateTimeSeries(ctx, parseFilters(r, tenantID, startMs, endMs))
	})
}

func (h *Handler) GetStatusTimeSeries(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query status time series", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetStatusTimeSeries(ctx, parseFilters(r, tenantID, startMs, endMs))
	})
}

func (h *Handler) GetLatencyPercentilesTimeSeries(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query latency percentiles", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetLatencyPercentilesTimeSeries(ctx, parseFilters(r, tenantID, startMs, endMs))
	})
}

func (h *Handler) GetREDByEndpointTimeSeries(w http.ResponseWriter, r *http.Request) {
	limit := httputil.ParsePageSize(r, "limit", 0)
	httputil.HandleRangeQuery(w, r, "Failed to query per-endpoint time series", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetREDByEndpointTimeSeries(ctx, parseFilters(r, tenantID, startMs, endMs), limit)
	})
}

func parseTopCursor(r *http.Request) models.TopEndpointsCursor {
	var cur models.TopEndpointsCursor
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		if decoded, ok := cursor.Decode[models.TopEndpointsCursor](raw); ok {
			cur = decoded
		}
	}
	return cur
}

func (h *Handler) GetTopEndpointsCombined(w http.ResponseWriter, r *http.Request) {
	limit, cur := httputil.ParsePageSize(r, "limit", 50), parseTopCursor(r)
	httputil.HandleComparableRangeQuery(w, r, "Failed to query top endpoints", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopEndpointsCombined(ctx, parseFilters(r, tenantID, startMs, endMs), limit, cur)
	})
}

func (h *Handler) GetTopDBQueriesCombined(w http.ResponseWriter, r *http.Request) {
	limit, cur := httputil.ParsePageSize(r, "limit", 50), parseTopCursor(r)
	httputil.HandleComparableRangeQuery(w, r, "Failed to query top db queries", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopDBQueries(ctx, parseFilters(r, tenantID, startMs, endMs), limit, cur)
	})
}

func (h *Handler) GetRequestRateTimeSeries(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query service request rate time series", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetRequestRateTimeSeries(ctx, parseFilters(r, tenantID, startMs, endMs))
	})
}

func (h *Handler) GetServiceSummary(w http.ResponseWriter, r *http.Request) {
	httputil.HandleComparableRangeQuery(w, r, "Failed to query service summary", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetServiceSummary(ctx, parseFilters(r, tenantID, startMs, endMs))
	})
}

func (h *Handler) GetOperationBaseline(w http.ResponseWriter, r *http.Request) {
	tenantID := httputil.Tenant(r).TenantID
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	serviceName := r.URL.Query().Get("service")
	operationName := r.URL.Query().Get("operation")
	if serviceName == "" || operationName == "" {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "service and operation are required", nil)
		return
	}
	resp, err := h.Service.GetOperationBaseline(r.Context(), tenantID, startMs, endMs, serviceName, operationName)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query operation baseline", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) GetServiceSaturationTimeSeries(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query service saturation time series", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetServiceSaturationTimeSeries(ctx, parseFilters(r, tenantID, startMs, endMs))
	})
}

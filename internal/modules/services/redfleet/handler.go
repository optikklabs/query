package redfleet

import (
	"context"
	"net/http"

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

// parseFilters scopes a RED query to the repeated services param; none
// means every service.
func parseFilters(r *http.Request, tenantID, startMs, endMs int64) filter.Filters {
	return filter.Filters{TenantID: tenantID, StartMs: startMs, EndMs: endMs, Services: r.URL.Query()["services"]}
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

const (
	defaultEndpointLimit = 20
	defaultTopLimit      = 50
	maxLimit             = 200
)

func (h *Handler) GetREDByEndpointTimeSeries(w http.ResponseWriter, r *http.Request) {
	limit, ok := httputil.QueryLimit(w, r, defaultEndpointLimit, maxLimit)
	if !ok {
		return
	}
	httputil.HandleRangeQuery(w, r, "Failed to query per-endpoint time series", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetREDByEndpointTimeSeries(ctx, parseFilters(r, tenantID, startMs, endMs), limit)
	})
}

// parseTopPage reads the top-N limit and page cursor, answering 400 when
// either is invalid.
func parseTopPage(w http.ResponseWriter, r *http.Request) (int, *models.TopEndpointsCursor, bool) {
	limit, ok := httputil.QueryLimit(w, r, defaultTopLimit, maxLimit)
	if !ok {
		return 0, nil, false
	}
	cur, err := cursor.Decode[models.TopEndpointsCursor](r.URL.Query().Get("cursor"))
	if err != nil {
		httputil.RespondServiceError(w, r, err, "invalid cursor")
		return 0, nil, false
	}
	return limit, cur, true
}

func (h *Handler) GetTopEndpointsCombined(w http.ResponseWriter, r *http.Request) {
	limit, cur, ok := parseTopPage(w, r)
	if !ok {
		return
	}
	httputil.HandleComparableRangeQuery(w, r, "Failed to query top endpoints", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopEndpointsCombined(ctx, parseFilters(r, tenantID, startMs, endMs), limit, cur)
	})
}

func (h *Handler) GetTopDBQueriesCombined(w http.ResponseWriter, r *http.Request) {
	limit, cur, ok := parseTopPage(w, r)
	if !ok {
		return
	}
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
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "service and operation are required", nil)
		return
	}
	resp, err := h.Service.GetOperationBaseline(r.Context(), tenantID, startMs, endMs, serviceName, operationName)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Failed to query operation baseline")
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) GetServiceSaturationTimeSeries(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query service saturation time series", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetServiceSaturationTimeSeries(ctx, parseFilters(r, tenantID, startMs, endMs))
	})
}

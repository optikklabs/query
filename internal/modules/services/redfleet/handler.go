package redfleet

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/shared/errorcode"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type REDFleetHandler struct {
	Service *Service
}

func parseREDFilters(r *http.Request, tenantID, startMs, endMs int64) REDFilters {
	f := REDFilters{TenantID: tenantID, StartMs: startMs, EndMs: endMs}
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

func (h *REDFleetHandler) GetFleetServices(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query fleet services", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetFleetServices(ctx, parseREDFilters(r, tenantID, startMs, endMs))
	})
}

func (h *REDFleetHandler) GetFleetOverview(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleComparableRangeQuery(w, r, "Failed to query fleet overview", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetFleetOverview(ctx, parseREDFilters(r, tenantID, startMs, endMs))
	})
}

func (h *REDFleetHandler) GetRequestAndErrorRateTimeSeries(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query request and error rate time series", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetRequestAndErrorRateTimeSeries(ctx, parseREDFilters(r, tenantID, startMs, endMs))
	})
}

func (h *REDFleetHandler) GetStatusTimeSeries(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query status time series", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetStatusTimeSeries(ctx, parseREDFilters(r, tenantID, startMs, endMs))
	})
}

func (h *REDFleetHandler) GetLatencyPercentilesTimeSeries(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query latency percentiles", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetLatencyPercentilesTimeSeries(ctx, parseREDFilters(r, tenantID, startMs, endMs))
	})
}

func (h *REDFleetHandler) GetREDByEndpointTimeSeries(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query per-endpoint time series", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetREDByEndpointTimeSeries(ctx, parseREDFilters(r, tenantID, startMs, endMs))
	})
}

func parseTopCursor(r *http.Request) TopEndpointsCursor {
	var cur TopEndpointsCursor
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		if decoded, ok := cursor.Decode[TopEndpointsCursor](raw); ok {
			cur = decoded
		}
	}
	return cur
}

func (h *REDFleetHandler) GetTopEndpointsCombined(w http.ResponseWriter, r *http.Request) {
	limit, cur := modulecommon.ParsePageSize(r, "limit", 50), parseTopCursor(r)
	modulecommon.HandleComparableRangeQuery(w, r, "Failed to query top endpoints", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopEndpointsCombined(ctx, parseREDFilters(r, tenantID, startMs, endMs), limit, cur)
	})
}

func (h *REDFleetHandler) GetTopDBQueriesCombined(w http.ResponseWriter, r *http.Request) {
	limit, cur := modulecommon.ParsePageSize(r, "limit", 50), parseTopCursor(r)
	modulecommon.HandleComparableRangeQuery(w, r, "Failed to query top db queries", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetTopDBQueries(ctx, parseREDFilters(r, tenantID, startMs, endMs), limit, cur)
	})
}

func (h *REDFleetHandler) GetRequestRateTimeSeries(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query service request rate time series", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetRequestRateTimeSeries(ctx, parseREDFilters(r, tenantID, startMs, endMs))
	})
}

func (h *REDFleetHandler) GetServiceSummary(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleComparableRangeQuery(w, r, "Failed to query service summary", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetServiceSummary(ctx, parseREDFilters(r, tenantID, startMs, endMs))
	})
}

func (h *REDFleetHandler) GetOperationBaseline(w http.ResponseWriter, r *http.Request) {
	tenantID := modulecommon.Tenant(r).TenantID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	serviceName := r.URL.Query().Get("service")
	operationName := r.URL.Query().Get("operation")
	if serviceName == "" || operationName == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "service and operation are required", nil)
		return
	}
	resp, err := h.Service.GetOperationBaseline(r.Context(), tenantID, startMs, endMs, serviceName, operationName)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query operation baseline", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *REDFleetHandler) GetServiceSaturationTimeSeries(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query service saturation time series", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetServiceSaturationTimeSeries(ctx, parseREDFilters(r, tenantID, startMs, endMs))
	})
}

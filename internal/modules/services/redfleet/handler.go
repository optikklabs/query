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

// parseREDFilters extracts REDFilters from any incoming request. It supports:
//   - ?serviceName=X         (single service, legacy)
//   - ?services=X&services=Y (multi-service, new)
//   - /{serviceName} path    (chi URL param, backwards compat)
func parseREDFilters(r *http.Request, teamID, startMs, endMs int64) REDFilters {
	f := REDFilters{TeamID: teamID, StartMs: startMs, EndMs: endMs}
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
	modulecommon.HandleRangeQuery(w, r, "Failed to query fleet services", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetFleetServices(ctx, parseREDFilters(r, teamID, startMs, endMs))
	})
}

func (h *REDFleetHandler) GetFleetOverview(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query fleet overview", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetFleetOverview(ctx, parseREDFilters(r, teamID, startMs, endMs))
	})
}

func (h *REDFleetHandler) GetRequestAndErrorRateTimeSeries(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	f := parseREDFilters(r, teamID, startMs, endMs)
	resp, err := modulecommon.WithComparison(r, startMs, endMs, func(s, e int64) (any, error) {
		cf := f
		cf.StartMs, cf.EndMs = s, e
		return h.Service.GetRequestAndErrorRateTimeSeries(r.Context(), cf)
	})
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query request and error rate time series", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

// GetStatusTimeSeries returns status split by HTTP family over time.
func (h *REDFleetHandler) GetStatusTimeSeries(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query status time series", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetStatusTimeSeries(ctx, parseREDFilters(r, teamID, startMs, endMs))
	})
}

func (h *REDFleetHandler) GetLatencyPercentilesTimeSeries(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query latency percentiles", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetLatencyPercentilesTimeSeries(ctx, parseREDFilters(r, teamID, startMs, endMs))
	})
}

func (h *REDFleetHandler) GetREDByEndpointTimeSeries(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query per-endpoint time series", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetREDByEndpointTimeSeries(ctx, parseREDFilters(r, teamID, startMs, endMs))
	})
}

func (h *REDFleetHandler) GetTopEndpointsCombined(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	f := parseREDFilters(r, teamID, startMs, endMs)
	limit := modulecommon.ParsePageSize(r, "limit", 50)
	cursorStr := r.URL.Query().Get("cursor")
	var cur TopEndpointsCursor
	if cursorStr != "" {
		if decoded, ok := cursor.Decode[TopEndpointsCursor](cursorStr); ok {
			cur = decoded
		}
	}
	resp, err := modulecommon.WithComparison(r, startMs, endMs, func(s, e int64) (any, error) {
		cf := f
		cf.StartMs, cf.EndMs = s, e
		return h.Service.GetTopEndpointsCombined(r.Context(), cf, limit, cur)
	})
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query top endpoints", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *REDFleetHandler) GetTopDBQueriesCombined(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	f := parseREDFilters(r, teamID, startMs, endMs)
	limit := modulecommon.ParsePageSize(r, "limit", 50)
	cursorStr := r.URL.Query().Get("cursor")
	var cur TopEndpointsCursor
	if cursorStr != "" {
		if decoded, ok := cursor.Decode[TopEndpointsCursor](cursorStr); ok {
			cur = decoded
		}
	}
	resp, err := modulecommon.WithComparison(r, startMs, endMs, func(s, e int64) (any, error) {
		cf := f
		cf.StartMs, cf.EndMs = s, e
		return h.Service.GetTopDBQueries(r.Context(), cf, limit, cur)
	})
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query top db queries", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *REDFleetHandler) GetRequestRateTimeSeries(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := h.Service.GetRequestRateTimeSeries(r.Context(), parseREDFilters(r, teamID, startMs, endMs))
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query service request rate time series", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *REDFleetHandler) GetServiceSummary(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	f := parseREDFilters(r, teamID, startMs, endMs)
	resp, err := modulecommon.WithComparison(r, startMs, endMs, func(s, e int64) (any, error) {
		cf := f
		cf.StartMs, cf.EndMs = s, e
		return h.Service.GetServiceSummary(r.Context(), cf)
	})
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query service summary", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

// GetOperationBaseline returns windowed p50/p95/p99 for service + operation.
func (h *REDFleetHandler) GetOperationBaseline(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	serviceName := r.URL.Query().Get("service")
	operationName := r.URL.Query().Get("operation")
	if serviceName == "" || operationName == "" {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.BadRequest, "service and operation are required")
		return
	}
	resp, err := h.Service.GetOperationBaseline(r.Context(), teamID, startMs, endMs, serviceName, operationName)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query operation baseline", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *REDFleetHandler) GetServiceSaturationTimeSeries(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query service saturation time series", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetServiceSaturationTimeSeries(ctx, parseREDFilters(r, teamID, startMs, endMs))
	})
}

package redfleet

import (
	"context"
	"net/http"

	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/shared/errorcode"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type REDFleetHandler struct {
	Service *Service
}

func (h *REDFleetHandler) GetFleetTotals(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query fleet totals", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetFleetTotals(ctx, teamID, startMs, endMs)
	})
}

func (h *REDFleetHandler) GetFleetServices(w http.ResponseWriter, r *http.Request) {
	modulecommon.HandleRangeQuery(w, r, "Failed to query fleet services", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetFleetServices(ctx, teamID, startMs, endMs)
	})
}

func (h *REDFleetHandler) GetApdex(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	satisfiedMs := modulecommon.ParseFloatParam(r, "satisfied_ms", 300.0)
	toleratingMs := modulecommon.ParseFloatParam(r, "tolerating_ms", 1200.0)
	if satisfiedMs <= 0 || toleratingMs <= 0 || satisfiedMs >= toleratingMs {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.BadRequest, "satisfied_ms must be positive and less than tolerating_ms")
		return
	}
	serviceName := r.URL.Query().Get("serviceName")
	resp, err := h.Service.GetApdex(r.Context(), teamID, startMs, endMs, satisfiedMs, toleratingMs, serviceName)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query Apdex scores", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *REDFleetHandler) GetRequestAndErrorRateTimeSeries(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := modulecommon.WithComparison(r, startMs, endMs, func(s, e int64) (any, error) {
		return h.Service.GetRequestAndErrorRateTimeSeries(r.Context(), teamID, s, e)
	})
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query request and error rate time series", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

// GetStatusTimeSeries returns status split by HTTP family over time.
func (h *REDFleetHandler) GetStatusTimeSeries(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("serviceName")
	modulecommon.HandleRangeQuery(w, r, "Failed to query status time series", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetStatusTimeSeries(ctx, teamID, startMs, endMs, serviceName)
	})
}

// GetLatencyPercentilesTimeSeries returns p50/p95/p99 latency over time.
func (h *REDFleetHandler) GetLatencyPercentilesTimeSeries(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("serviceName")
	modulecommon.HandleRangeQuery(w, r, "Failed to query latency percentiles", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetLatencyPercentilesTimeSeries(ctx, teamID, startMs, endMs, serviceName)
	})
}

// GetTopEndpointsCombined returns per-operation metrics for the endpoints.
func (h *REDFleetHandler) GetTopEndpointsCombined(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	serviceName := r.URL.Query().Get("serviceName")
	limit := modulecommon.ParsePageSize(r, "limit", 50)
	cursorStr := r.URL.Query().Get("cursor")
	var cur TopEndpointsCursor
	if cursorStr != "" {
		if decoded, ok := cursor.Decode[TopEndpointsCursor](cursorStr); ok {
			cur = decoded
		}
	}
	resp, err := modulecommon.WithComparison(r, startMs, endMs, func(s, e int64) (any, error) {
		return h.Service.GetTopEndpointsCombined(r.Context(), teamID, s, e, serviceName, limit, cur)
	})
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query top endpoints", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

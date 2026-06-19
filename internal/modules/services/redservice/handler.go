package redservice

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/shared/errorcode"

	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type REDServiceHandler struct {
	Service *Service
}

func (h *REDServiceHandler) GetServiceSummary(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	serviceName := chi.URLParam(r, "serviceName")
	resp, err := modulecommon.WithComparison(r, startMs, endMs, func(s, e int64) (any, error) {
		return h.Service.GetServiceSummary(r.Context(), teamID, s, e, serviceName)
	})
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query service summary", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

// GetOperationBaseline returns windowed p50/p95/p99 for service + operation.
func (h *REDServiceHandler) GetOperationBaseline(w http.ResponseWriter, r *http.Request) {
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

func (h *REDServiceHandler) GetServiceSaturationTimeSeries(w http.ResponseWriter, r *http.Request) {
	serviceName := chi.URLParam(r, "serviceName")
	modulecommon.HandleRangeQuery(w, r, "Failed to query service saturation time series", func(ctx context.Context, teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetServiceSaturationTimeSeries(ctx, teamID, startMs, endMs, serviceName)
	})
}

package traces

import (
	"net/http"

	"github.com/optikklabs/query/internal/modules/traces/service"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

const (
	defaultRelatedLimit = 10
	maxRelatedLimit     = 50
)

type Handler struct {
	Service *service.Service
}

func traceScope(w http.ResponseWriter, r *http.Request) (tenantID int64, traceID string, startMs, endMs int64, ok bool) {
	traceID = httputil.URLParamLower(r, "traceId")
	if traceID == "" {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required", nil)
		return 0, "", 0, 0, false
	}
	startMs, endMs, ok = httputil.ParseRequiredRange(w, r)
	return httputil.Tenant(r).TenantID, traceID, startMs, endMs, ok
}

// GetTraceDetail serves the consolidated trace view: summary, span list
// and all derived views in one response. A trace with no spans in range
// responds 200 with a nil summary and empty spans, matching the previous
// /spans behaviour the UI's logs-only fallback relies on.
func (h *Handler) GetTraceDetail(w http.ResponseWriter, r *http.Request) {
	tenantID, traceID, startMs, endMs, ok := traceScope(w, r)
	if !ok {
		return
	}
	detail, err := h.Service.GetTraceDetail(r.Context(), tenantID, traceID, startMs, endMs)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Failed to fetch trace")
		return
	}
	httputil.RespondOK(w, detail)
}

func (h *Handler) GetSpanEvents(w http.ResponseWriter, r *http.Request) {
	tenantID, traceID, startMs, endMs, ok := traceScope(w, r)
	if !ok {
		return
	}
	events, err := h.Service.GetSpanEvents(r.Context(), tenantID, traceID, startMs, endMs)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Failed to query span events")
		return
	}
	httputil.RespondOK(w, events)
}

func (h *Handler) GetSpanAttributes(w http.ResponseWriter, r *http.Request) {
	tenantID, traceID, startMs, endMs, ok := traceScope(w, r)
	if !ok {
		return
	}
	spanID := httputil.URLParamLower(r, "spanId")
	if spanID == "" {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "spanId is required", nil)
		return
	}
	attrs, err := h.Service.GetSpanAttributes(r.Context(), tenantID, traceID, spanID, startMs, endMs)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Failed to query span attributes")
		return
	}
	httputil.RespondOK(w, attrs)
}

func (h *Handler) GetRelatedTraces(w http.ResponseWriter, r *http.Request) {
	tenantID, traceID, startMs, endMs, ok := traceScope(w, r)
	if !ok {
		return
	}
	serviceName := r.URL.Query().Get("service")
	operationName := r.URL.Query().Get("operation")
	if serviceName == "" || operationName == "" {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "service and operation are required", nil)
		return
	}
	limit, ok := httputil.QueryLimit(w, r, defaultRelatedLimit, maxRelatedLimit)
	if !ok {
		return
	}

	traces, err := h.Service.GetRelatedTraces(r.Context(), tenantID, serviceName, operationName, startMs, endMs, traceID, limit)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Failed to query related traces")
		return
	}
	httputil.RespondOK(w, traces)
}

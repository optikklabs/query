package traces

import (
	"net/http"

	"github.com/optikklabs/query/internal/modules/traces/service"
	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

const (
	defaultRelatedLimit = 10
	maxRelatedLimit     = 50
)

type Handler struct {
	Service *service.Service
}

func traceScope(w http.ResponseWriter, r *http.Request) (tenantID int64, traceID string, startMs, endMs int64, ok bool) {
	traceID = modulecommon.URLParamLower(r, "traceId")
	if traceID == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required", nil)
		return 0, "", 0, 0, false
	}
	startMs, endMs, err := modulecommon.ParseRange(r)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, err.Error(), nil)
		return 0, "", 0, 0, false
	}
	return modulecommon.Tenant(r).TenantID, traceID, startMs, endMs, true
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
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to fetch trace", err)
		return
	}
	modulecommon.RespondOK(w, detail)
}

func (h *Handler) GetSpanEvents(w http.ResponseWriter, r *http.Request) {
	tenantID, traceID, startMs, endMs, ok := traceScope(w, r)
	if !ok {
		return
	}
	events, err := h.Service.GetSpanEvents(r.Context(), tenantID, traceID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query span events", err)
		return
	}
	modulecommon.RespondOK(w, events)
}

func (h *Handler) GetSpanAttributes(w http.ResponseWriter, r *http.Request) {
	tenantID, traceID, startMs, endMs, ok := traceScope(w, r)
	if !ok {
		return
	}
	spanID := modulecommon.URLParamLower(r, "spanId")
	if spanID == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "spanId is required", nil)
		return
	}
	attrs, err := h.Service.GetSpanAttributes(r.Context(), tenantID, traceID, spanID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query span attributes", err)
		return
	}
	if attrs == nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusNotFound, errorcode.NotFound, "Span not found", nil)
		return
	}
	modulecommon.RespondOK(w, attrs)
}

func (h *Handler) GetRelatedTraces(w http.ResponseWriter, r *http.Request) {
	tenantID := modulecommon.Tenant(r).TenantID
	traceID := modulecommon.URLParamLower(r, "traceId")
	if traceID == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required", nil)
		return
	}
	serviceName := r.URL.Query().Get("service")
	operationName := r.URL.Query().Get("operation")

	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	if serviceName == "" || operationName == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "service and operation are required", nil)
		return
	}

	limit := modulecommon.ParseIntParam(r, "limit", defaultRelatedLimit)
	if limit <= 0 || limit > maxRelatedLimit {
		limit = defaultRelatedLimit
	}

	traces, err := h.Service.GetRelatedTraces(r.Context(), tenantID, serviceName, operationName, startMs, endMs, traceID, limit)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query related traces", err)
		return
	}
	modulecommon.RespondOK(w, traces)
}

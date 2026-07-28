package traces

import (
	"net/http"

	"github.com/optikklabs/query/internal/modules/traces/models"
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

func (h *Handler) GetTraceSummary(w http.ResponseWriter, r *http.Request) {
	tenantID, traceID, startMs, endMs, ok := traceScope(w, r)
	if !ok {
		return
	}
	resp, err := h.Service.GetTraceSummary(r.Context(), tenantID, traceID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to fetch trace", err)
		return
	}
	if resp == nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusNotFound, errorcode.NotFound, "trace not found", nil)
		return
	}
	modulecommon.RespondOK(w, resp)
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

func (h *Handler) GetTraceSpans(w http.ResponseWriter, r *http.Request) {
	tenantID, traceID, startMs, endMs, ok := traceScope(w, r)
	if !ok {
		return
	}
	items, err := h.Service.ListSpansByTrace(r.Context(), tenantID, traceID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to list trace spans", err)
		return
	}
	if items == nil {
		items = []models.SpanListItem{}
	}
	modulecommon.RespondOK(w, map[string]any{"spans": items})
}

func (h *Handler) GetCriticalPath(w http.ResponseWriter, r *http.Request) {
	tenantID, traceID, startMs, endMs, ok := traceScope(w, r)
	if !ok {
		return
	}
	path, err := h.Service.GetCriticalPath(r.Context(), tenantID, traceID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to compute critical path", err)
		return
	}
	modulecommon.RespondOK(w, path)
}

func (h *Handler) GetErrorPath(w http.ResponseWriter, r *http.Request) {
	tenantID, traceID, startMs, endMs, ok := traceScope(w, r)
	if !ok {
		return
	}
	path, err := h.Service.GetErrorPath(r.Context(), tenantID, traceID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to compute error path", err)
		return
	}
	modulecommon.RespondOK(w, path)
}

func (h *Handler) GetServiceMap(w http.ResponseWriter, r *http.Request) {
	tenantID, traceID, startMs, endMs, ok := traceScope(w, r)
	if !ok {
		return
	}
	resp, err := h.Service.GetServiceMap(r.Context(), tenantID, traceID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to compute service map", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) GetTraceErrors(w http.ResponseWriter, r *http.Request) {
	tenantID, traceID, startMs, endMs, ok := traceScope(w, r)
	if !ok {
		return
	}
	groups, err := h.Service.GetTraceErrors(r.Context(), tenantID, traceID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to fetch trace errors", err)
		return
	}
	modulecommon.RespondOK(w, groups)
}

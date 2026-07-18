package llm

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Apps(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.Apps(r.Context(), modulecommon.Tenant(r).TenantID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM apps", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.Overview(r.Context(), modulecommon.Tenant(r).TenantID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM overview", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

var timeseriesMetrics = map[string]struct{}{
	"tokens_by_vendor": {},
	"latency":          {},
	"spend":            {},
}

func (h *Handler) Timeseries(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	metric := r.URL.Query().Get("metric")
	if _, valid := timeseriesMetrics[metric]; !valid {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "metric must be one of tokens_by_vendor, latency, spend", nil)
		return
	}
	resp, err := h.svc.Timeseries(r.Context(), modulecommon.Tenant(r).TenantID, startMs, endMs, metric)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM timeseries", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) CostBreakdown(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	groupBy := r.URL.Query().Get("groupBy")
	switch groupBy {
	case "":
		groupBy = "service"
	case "service", "vendor", "model":
	default:
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "groupBy must be one of service, vendor, model", nil)
		return
	}
	resp, err := h.svc.CostBreakdown(r.Context(), modulecommon.Tenant(r).TenantID, startMs, endMs, groupBy)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM cost breakdown", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) TracesQuery(w http.ResponseWriter, r *http.Request) {
	var req TracesQueryRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body", nil)
		return
	}
	if req.StartTime <= 0 || req.EndTime <= 0 || req.StartTime >= req.EndTime {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Valid startTime and endTime are required", nil)
		return
	}
	resp, err := h.svc.QueryTraces(r.Context(), modulecommon.Tenant(r).TenantID, req)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM traces", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) TraceDetail(w http.ResponseWriter, r *http.Request) {
	traceID := chi.URLParam(r, "traceId")
	if traceID == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "traceId is required", nil)
		return
	}
	startTimeMs, endTimeMs, ok := modulecommon.ParseRequiredExplicitRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.TraceDetail(r.Context(), modulecommon.Tenant(r).TenantID, traceID, startTimeMs, endTimeMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to load LLM trace", err)
		return
	}
	if resp.TraceID == "" || len(resp.Spans) == 0 {
		modulecommon.RespondErrorWithCause(w, r, http.StatusNotFound, errorcode.NotFound, "Trace not found", nil)
		return
	}
	modulecommon.RespondOK(w, resp)
}

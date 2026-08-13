package llm

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/modules/llm/pricing"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Apps(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.Apps(r.Context(), httputil.Tenant(r).TenantID, startMs, endMs)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM apps", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) Models(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.Models(r.Context(), httputil.Tenant(r).TenantID, startMs, endMs)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM models", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) Pricing(w http.ResponseWriter, r *http.Request) {
	httputil.RespondOK(w, map[string]any{"models": pricing.Table()})
}

func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.Overview(r.Context(), httputil.Tenant(r).TenantID, startMs, endMs)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM overview", err)
		return
	}
	httputil.RespondOK(w, resp)
}

var timeseriesMetrics = map[string]struct{}{
	"tokens_by_vendor": {},
	"latency":          {},
	"spend":            {},
}

func (h *Handler) Timeseries(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	metric := r.URL.Query().Get("metric")
	if _, valid := timeseriesMetrics[metric]; !valid {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "metric must be one of tokens_by_vendor, latency, spend", nil)
		return
	}
	resp, err := h.svc.Timeseries(r.Context(), httputil.Tenant(r).TenantID, startMs, endMs, metric)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM timeseries", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) CostBreakdown(w http.ResponseWriter, r *http.Request) {
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	groupBy := r.URL.Query().Get("groupBy")
	switch groupBy {
	case "":
		groupBy = "service"
	case "service", "vendor", "model":
	default:
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "groupBy must be one of service, vendor, model", nil)
		return
	}
	resp, err := h.svc.CostBreakdown(r.Context(), httputil.Tenant(r).TenantID, startMs, endMs, groupBy)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM cost breakdown", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) TracesQuery(w http.ResponseWriter, r *http.Request) {
	var req TracesQueryRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body", nil)
		return
	}
	if req.StartTime <= 0 || req.EndTime <= 0 || req.StartTime >= req.EndTime {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Valid startTime and endTime are required", nil)
		return
	}
	resp, err := h.svc.QueryTraces(r.Context(), httputil.Tenant(r).TenantID, req)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query LLM traces", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) TraceDetail(w http.ResponseWriter, r *http.Request) {
	traceID := chi.URLParam(r, "traceId")
	if traceID == "" {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "traceId is required", nil)
		return
	}
	startTimeMs, endTimeMs, ok := httputil.ParseRequiredExplicitRange(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.TraceDetail(r.Context(), httputil.Tenant(r).TenantID, traceID, startTimeMs, endTimeMs)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to load LLM trace", err)
		return
	}
	if resp.TraceID == "" || len(resp.Spans) == 0 {
		httputil.RespondErrorWithCause(w, r, http.StatusNotFound, errorcode.NotFound, "Trace not found", nil)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) SpanIO(w http.ResponseWriter, r *http.Request) {
	traceID := chi.URLParam(r, "traceId")
	spanID := chi.URLParam(r, "spanId")
	if traceID == "" || spanID == "" {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "traceId and spanId are required", nil)
		return
	}
	startTimeMs, endTimeMs, ok := httputil.ParseRequiredExplicitRange(w, r)
	if !ok {
		return
	}
	resp, found, err := h.svc.SpanIO(r.Context(), httputil.Tenant(r).TenantID, traceID, spanID, startTimeMs, endTimeMs)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to load LLM span content", err)
		return
	}
	if !found {
		httputil.RespondErrorWithCause(w, r, http.StatusNotFound, errorcode.NotFound, "Span not found", nil)
		return
	}
	httputil.RespondOK(w, resp)
}

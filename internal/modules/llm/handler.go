package llm

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
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

func (h *Handler) TracesQuery(w http.ResponseWriter, r *http.Request) {
	var req TracesQueryRequest
	if !httputil.BindJSON(w, r, &req) {
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
	startTimeMs, endTimeMs, ok := httputil.ParseRequiredUncappedRange(w, r)
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
	startTimeMs, endTimeMs, ok := httputil.ParseRequiredUncappedRange(w, r)
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

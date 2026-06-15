package detail

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

const defaultRelatedLimit = 10

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{
		svc: svc,
	}
}

func (h *Handler) GetTraceSummary(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	traceID := chi.URLParam(r, "traceId")
	if traceID == "" {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required")
		return
	}
	resp, err := h.svc.GetTraceSummary(r.Context(), teamID, traceID)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to fetch trace", err)
		return
	}
	if resp == nil {
		modulecommon.RespondError(w, r, http.StatusNotFound, errorcode.NotFound, "trace not found")
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) GetSpanEvents(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	traceID := chi.URLParam(r, "traceId")
	if traceID == "" {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required")
		return
	}
	events, err := h.svc.GetSpanEvents(r.Context(), teamID, traceID)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query span events", err)
		return
	}
	modulecommon.RespondOK(w, events)
}

func (h *Handler) GetSpanAttributes(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	traceID := chi.URLParam(r, "traceId")
	spanID := chi.URLParam(r, "spanId")
	if traceID == "" {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required")
		return
	}
	if spanID == "" {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "spanId is required")
		return
	}

	attrs, err := h.svc.GetSpanAttributes(r.Context(), teamID, traceID, spanID)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query span attributes", err)
		return
	}
	if attrs == nil {
		modulecommon.RespondError(w, r, http.StatusNotFound, errorcode.NotFound, "Span not found")
		return
	}
	modulecommon.RespondOK(w, attrs)
}

func (h *Handler) GetRelatedTraces(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	traceID := chi.URLParam(r, "traceId")
	if traceID == "" {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required")
		return
	}
	serviceName := r.URL.Query().Get("service")
	operationName := r.URL.Query().Get("operation")

	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	if serviceName == "" || operationName == "" {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "service and operation are required")
		return
	}

	limit := modulecommon.ParseIntParam(r, "limit", defaultRelatedLimit)
	if limit <= 0 || limit > 50 {
		limit = defaultRelatedLimit
	}

	traces, err := h.svc.GetRelatedTraces(r.Context(), teamID, serviceName, operationName, startMs, endMs, traceID, limit)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query related traces", err)
		return
	}
	modulecommon.RespondOK(w, traces)
}

func (h *Handler) GetTraceSpans(w http.ResponseWriter, r *http.Request) {
	teamID := modulecommon.Tenant(r).TeamID
	traceID := chi.URLParam(r, "traceId")
	if traceID == "" {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required")
		return
	}
	items, err := h.svc.ListSpansByTrace(r.Context(), teamID, traceID)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to list trace spans", err)
		return
	}
	if items == nil {
		items = []SpanListItem{}
	}
	modulecommon.RespondOK(w, map[string]any{"spans": items})
}

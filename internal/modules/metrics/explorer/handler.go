package explorer

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func (h *Handler) ListMetricNames(w http.ResponseWriter, r *http.Request) {
	tenantID := httputil.Tenant(r).TenantID
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	search := r.URL.Query().Get("search")

	entries, err := h.Service.ListMetricNames(r.Context(), tenantID, startMs, endMs, search)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to list metric names", err)
		return
	}

	httputil.RespondOK(w, MetricNamesResponse{Metrics: entries})
}

func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	tenantID := httputil.Tenant(r).TenantID
	startMs, endMs, ok := httputil.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	metricName := chi.URLParam(r, "metricName")
	if metricName == "" {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "metricName is required", nil)
		return
	}
	tagKey := r.URL.Query().Get("tagKey")

	tags, err := h.Service.ListTags(r.Context(), tenantID, startMs, endMs, metricName, tagKey)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Failed to list tags")
		return
	}
	httputil.RespondOK(w, TagsResponse{Tags: tags})
}

func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	var req QueryRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "Invalid request body", nil)
		return
	}
	if err := validateQueryRequest(req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, err.Error(), nil)
		return
	}

	tenantID := httputil.Tenant(r).TenantID
	result, err := h.Service.Query(r.Context(), tenantID, req)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Failed to execute explorer query")
		return
	}
	httputil.RespondOK(w, result)
}

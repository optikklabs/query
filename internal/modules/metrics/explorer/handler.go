package explorer

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func (h *Handler) ListMetricNames(w http.ResponseWriter, r *http.Request) {
	tenantID := modulecommon.Tenant(r).TenantID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	search := r.URL.Query().Get("search")

	results, err := h.Service.ListMetricNames(r.Context(), tenantID, startMs, endMs, search)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to list metric names", err)
		return
	}

	entries := make([]FEMetricNameEntry, len(results))
	for i, r := range results {
		entries[i] = FEMetricNameEntry{
			Name:        r.MetricName,
			Type:        r.MetricType,
			Unit:        r.Unit,
			Description: r.Description,
		}
	}
	modulecommon.RespondOK(w, FEMetricNamesResponse{Metrics: entries})
}

func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	tenantID := modulecommon.Tenant(r).TenantID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	metricName := chi.URLParam(r, "metricName")
	if metricName == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "metricName is required", nil)
		return
	}
	tagKey := r.URL.Query().Get("tagKey")

	tags, err := h.Service.ListTags(r.Context(), tenantID, startMs, endMs, metricName, tagKey)
	if err != nil {
		modulecommon.RespondServiceError(w, r, err, "Failed to list tags")
		return
	}
	modulecommon.RespondOK(w, FETagsResponse{Tags: tags})
}

func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	var req FEQueryRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "Invalid request body", nil)
		return
	}
	if err := validateQueryRequest(req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, err.Error(), nil)
		return
	}

	tenantID := modulecommon.Tenant(r).TenantID
	result, err := h.Service.Query(r.Context(), tenantID, req)
	if err != nil {
		modulecommon.RespondServiceError(w, r, err, "Failed to execute explorer query")
		return
	}
	modulecommon.RespondOK(w, result)
}

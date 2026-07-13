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

// ListMetricNames handles GET /metrics/names
// Frontend expects: { "metrics": [{ "name", "type", "unit", "description" }] }
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
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to list tags", err)
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
	if req.StartTime <= 0 || req.EndTime <= 0 {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "startTime and endTime are required", nil)
		return
	}
	if len(req.Queries) == 0 {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "At least one query is required", nil)
		return
	}

	tenantID := modulecommon.Tenant(r).TenantID
	result, err := h.Service.Query(r.Context(), tenantID, req)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to execute explorer query", err)
		return
	}
	modulecommon.RespondOK(w, result)
}

package explorer

import (
	"net/http"
	"strings"

	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{
		svc: svc,
	}
}

func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	var req QueryRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body", nil)
		return
	}
	req.Filters.TenantID = modulecommon.Tenant(r).TenantID
	req.Filters.StartMs = req.StartTime
	req.Filters.EndMs = req.EndTime
	if err := req.Filters.Validate(); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid filters", err)
		return
	}
	resp, err := h.svc.Query(r.Context(), req)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query traces", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) QueryFacets(w http.ResponseWriter, r *http.Request) {
	var req FacetsRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body", nil)
		return
	}
	req.Filters.TenantID = modulecommon.Tenant(r).TenantID
	req.Filters.StartMs = req.StartTime
	req.Filters.EndMs = req.EndTime
	if err := req.Filters.Validate(); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid filters", err)
		return
	}
	resp, err := h.svc.QueryFacets(r.Context(), req)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query trace facets", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) QueryTrend(w http.ResponseWriter, r *http.Request) {
	var req TrendRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body", nil)
		return
	}
	req.Filters.TenantID = modulecommon.Tenant(r).TenantID
	req.Filters.StartMs = req.StartTime
	req.Filters.EndMs = req.EndTime
	if err := req.Filters.Validate(); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid filters", err)
		return
	}
	resp, err := h.svc.QueryTrend(r.Context(), req)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query trace trend", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) Suggest(w http.ResponseWriter, r *http.Request) {
	var req SuggestRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body", nil)
		return
	}
	if req.StartTime <= 0 || req.EndTime <= 0 || req.StartTime >= req.EndTime {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Valid startTime and endTime are required", nil)
		return
	}
	req.Field = strings.TrimSpace(req.Field)
	if req.Field == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "field is required", nil)
		return
	}
	if !strings.HasPrefix(req.Field, "@") && !IsScalarField(req.Field) {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "unknown field", nil)
		return
	}
	resp, err := h.svc.Suggest(r.Context(), req, modulecommon.Tenant(r).TenantID)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to fetch suggestions", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

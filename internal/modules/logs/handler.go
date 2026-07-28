package logs

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/modules/logs/models"
	"github.com/optikklabs/query/internal/modules/logs/service"
	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

const (
	defaultTraceLogsLimit = 1000
	maxTraceLogsLimit     = 5000
)

const maxQueryLimit = 5000

type Handler struct {
	Service *service.Service
}

func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	var req models.QueryRequest
	if !modulecommon.BindFiltered(w, r, &req) {
		return
	}
	if req.Limit <= 0 || req.Limit > maxQueryLimit {
		req.Limit = maxQueryLimit
	}
	resp, err := h.Service.Query(r.Context(), req)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query logs", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) Suggest(w http.ResponseWriter, r *http.Request) {
	var req models.SuggestRequest
	if !modulecommon.BindSuggestRequest(w, r, &req, service.IsSuggestableScalarField) {
		return
	}
	resp, err := h.Service.Suggest(r.Context(), req, modulecommon.Tenant(r).TenantID)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to fetch suggestions", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) Facets(w http.ResponseWriter, r *http.Request) {
	var req models.FacetsRequest
	if !modulecommon.BindFiltered(w, r, &req) {
		return
	}
	resp, err := h.Service.FacetsResponse(r.Context(), req.Filters)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query logs facets", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	var req models.TrendsRequest
	if !modulecommon.BindFiltered(w, r, &req) {
		return
	}
	sum, err := h.Service.Summary(r.Context(), req.Filters)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query logs summary", err)
		return
	}
	modulecommon.RespondOK(w, models.SummaryResponse{Summary: sum})
}

func (h *Handler) Trend(w http.ResponseWriter, r *http.Request) {
	var req models.TrendsRequest
	if !modulecommon.BindFiltered(w, r, &req) {
		return
	}
	tr, err := h.Service.Trend(r.Context(), req.Filters)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query logs trend", err)
		return
	}
	modulecommon.RespondOK(w, models.TrendResponse{Trend: tr})
}

func (h *Handler) GetByTrace(w http.ResponseWriter, r *http.Request) {
	traceID := modulecommon.URLParamLower(r, "traceID")
	if traceID == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "trace id required", nil)
		return
	}
	limit := modulecommon.ParseIntParam(r, "limit", defaultTraceLogsLimit)
	if limit <= 0 {
		limit = defaultTraceLogsLimit
	}
	if limit > maxTraceLogsLimit {
		limit = maxTraceLogsLimit
	}
	logs, err := h.Service.GetByTraceID(r.Context(), modulecommon.Tenant(r).TenantID, traceID, limit)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to fetch logs by trace", err)
		return
	}
	modulecommon.RespondOK(w, logs)
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "log id required", nil)
		return
	}
	startMs, endMs, err := modulecommon.ParseRange(r)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, err.Error(), nil)
		return
	}
	resp, err := h.Service.GetByID(r.Context(), modulecommon.Tenant(r).TenantID, id, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to fetch log", err)
		return
	}
	if resp == nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusNotFound, errorcode.NotFound, "log not found", nil)
		return
	}
	modulecommon.RespondOK(w, resp)
}

package explorer

import (
	"net/http"

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
	if !modulecommon.BindFiltered(w, r, &req) {
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
	if !modulecommon.BindFiltered(w, r, &req) {
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
	if !modulecommon.BindFiltered(w, r, &req) {
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
	if !modulecommon.BindSuggestRequest(w, r, &req, IsScalarField) {
		return
	}
	resp, err := h.svc.Suggest(r.Context(), req, modulecommon.Tenant(r).TenantID)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to fetch suggestions", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

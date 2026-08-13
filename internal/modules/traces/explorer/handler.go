package explorer

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
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
	if !httputil.BindFiltered(w, r, &req) {
		return
	}
	resp, err := h.svc.Query(r.Context(), req)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query traces", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) QueryFacets(w http.ResponseWriter, r *http.Request) {
	var req FacetsRequest
	if !httputil.BindFiltered(w, r, &req) {
		return
	}
	resp, err := h.svc.QueryFacets(r.Context(), req)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query trace facets", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) QueryTrend(w http.ResponseWriter, r *http.Request) {
	var req TrendRequest
	if !httputil.BindFiltered(w, r, &req) {
		return
	}
	resp, err := h.svc.QueryTrend(r.Context(), req)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query trace trend", err)
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) Suggest(w http.ResponseWriter, r *http.Request) {
	var req SuggestRequest
	if !httputil.BindSuggestRequest(w, r, &req, IsScalarField) {
		return
	}
	resp, err := h.svc.Suggest(r.Context(), req, httputil.Tenant(r).TenantID)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to fetch suggestions", err)
		return
	}
	httputil.RespondOK(w, resp)
}

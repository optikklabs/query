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

// Query powers POST /api/v1/logs/query (list + optional include blocks).
func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	var req QueryRequest
	if !modulecommon.BindFiltered(w, r, &req) {
		return
	}
	if req.Limit <= 0 || req.Limit > 5000 {
		req.Limit = 5000
	}
	resp, err := h.svc.Query(r.Context(), req)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query logs", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) Suggest(w http.ResponseWriter, r *http.Request) {
	var req SuggestRequest
	if !modulecommon.BindSuggestRequest(w, r, &req, IsSuggestableScalarField) {
		return
	}
	resp, err := h.svc.Suggest(r.Context(), req, modulecommon.Tenant(r).TenantID)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to fetch suggestions", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

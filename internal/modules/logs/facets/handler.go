package logfacets //nolint:revive,stylecheck

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

// Facets powers POST /api/v1/logs/facets.
func (h *Handler) Facets(w http.ResponseWriter, r *http.Request) {
	var req Request
	if !modulecommon.BindFiltered(w, r, &req) {
		return
	}
	resp, err := h.svc.ComputeResponse(r.Context(), req.Filters)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to query logs facets", err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

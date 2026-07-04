package onboarding

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Status powers GET /api/v1/onboarding/status.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	if tenant.TeamID == 0 {
		httputil.RespondError(w, r, http.StatusBadRequest, errorcode.BadRequest, "A team context is required")
		return
	}
	resp, err := h.svc.Status(r.Context(), tenant.TeamID)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "Failed to load onboarding status", err)
		return
	}
	httputil.RespondOK(w, resp)
}

package playground

import (
	"errors"
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	httputil "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	var req CompleteRequest
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	res, err := h.svc.Complete(r.Context(), httputil.Tenant(r).TenantID, req)
	switch {
	case errors.Is(err, errProviderFailed):
		httputil.RespondErrorWithCause(w, r, http.StatusBadGateway, errorcode.Internal, "provider request failed", err)
	case err != nil:
		httputil.RespondServiceError(w, r, err, "playground request failed")
	default:
		httputil.RespondOK(w, res)
	}
}

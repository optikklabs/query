package playground

import (
	"errors"
	"net/http"

	"github.com/optikklabs/query/internal/infra/llmproviders"
	"github.com/optikklabs/query/internal/shared/errorcode"
	httputil "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	var req CompleteRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "invalid request body", nil)
		return
	}
	res, err := h.svc.Complete(r.Context(), httputil.Tenant(r).TenantID, req)
	if err != nil {
		respondErr(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func respondErr(w http.ResponseWriter, r *http.Request, err error) {
	var ve ErrValidation
	switch {
	case errors.As(err, &ve):
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, ve.Msg, nil)
	case IsUnavailable(err):
		httputil.RespondErrorWithCause(w, r, http.StatusServiceUnavailable, errorcode.Unavailable, "no provider key configured for this provider", nil)
	case errors.Is(err, llmproviders.ErrUnknownProvider):
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "unsupported provider", nil)
	default:
		httputil.RespondErrorWithCause(w, r, http.StatusBadGateway, errorcode.Internal, "provider request failed", err)
	}
}

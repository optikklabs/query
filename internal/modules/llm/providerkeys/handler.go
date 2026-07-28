package providerkeys

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/shared/errorcode"
	httputil "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.List(r.Context(), httputil.Tenant(r).TenantID)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.QueryFailed, "failed to list provider keys", err)
		return
	}
	httputil.RespondOK(w, map[string]any{"items": res})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "invalid request body", nil)
		return
	}
	tenant := httputil.Tenant(r)
	res, err := h.svc.Create(r.Context(), tenant.TenantID, tenant.UserID, req)
	if err != nil {
		respondErr(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "invalid id", nil)
		return
	}
	if err := h.svc.Delete(r.Context(), httputil.Tenant(r).TenantID, id); err != nil {
		respondErr(w, r, err)
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": id})
}

func respondErr(w http.ResponseWriter, r *http.Request, err error) {
	var ve ErrValidation
	switch {
	case errors.Is(err, ErrNotFound):
		httputil.RespondErrorWithCause(w, r, http.StatusNotFound, errorcode.NotFound, "provider key not found", nil)
	case errors.Is(err, ErrNoEncryption):
		httputil.RespondErrorWithCause(w, r, http.StatusServiceUnavailable, errorcode.Unavailable, "provider key encryption is not configured", nil)
	case errors.As(err, &ve):
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, ve.Msg, nil)
	default:
		httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "provider key request failed", err)
	}
}

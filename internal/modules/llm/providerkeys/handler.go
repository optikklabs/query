package providerkeys

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
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), httputil.Tenant(r).TenantID, id); err != nil {
		respondErr(w, r, err)
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": id})
}

func respondErr(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrNoEncryption) {
		httputil.RespondErrorWithCause(w, r, http.StatusServiceUnavailable, errorcode.Unavailable, "provider key encryption is not configured", nil)
		return
	}
	httputil.RespondServiceError(w, r, err, "provider key request failed")
}

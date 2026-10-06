package providerkeys

import (
	"net/http"

	httputil "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.List(r.Context(), httputil.Tenant(r).TenantID)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "failed to list provider keys")
		return
	}
	httputil.RespondOK(w, map[string]any{"items": res})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	tenant := httputil.Tenant(r)
	res, err := h.svc.Create(r.Context(), tenant.TenantID, tenant.UserID, req)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "provider key request failed")
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
		httputil.RespondServiceError(w, r, err, "provider key request failed")
		return
	}
	httputil.RespondOK(w, map[string]any{"deleted": id})
}

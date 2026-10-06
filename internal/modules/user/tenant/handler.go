package tenant

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{Service: service}
}

func (h *Handler) IngestionEndpoints(w http.ResponseWriter, r *http.Request) {
	httputil.RespondOK(w, h.Service.IngestionEndpoints())
}

func (h *Handler) RotateAPIKey(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Service.RotateAPIKey(r.Context(), httputil.Tenant(r).TenantID)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Unable to rotate api key")
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Service.RevokeAPIKey(r.Context(), httputil.Tenant(r).TenantID)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Unable to revoke api key")
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) DeactivateTenant(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Service.DeactivateTenant(r.Context(), httputil.Tenant(r).TenantID)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Unable to deactivate tenant")
		return
	}
	httputil.RespondOK(w, resp)
}

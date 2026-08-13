package tenant

import (
	"net/http"

	"github.com/optikklabs/query/internal/modules/user/shared"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{Service: service}
}

func (h *Handler) IngestionEndpoints(w http.ResponseWriter, r *http.Request) {
	if httputil.Tenant(r).TenantID == 0 {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "A tenant context is required", nil)
		return
	}
	httputil.RespondOK(w, h.Service.IngestionEndpoints())
}

func (h *Handler) RotateAPIKey(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	if !h.requireTenantAdmin(w, r, tenant.TenantID, tenant.UserRole) {
		return
	}
	resp, err := h.Service.RotateAPIKey(r.Context(), tenant.TenantID)
	if err != nil {
		shared.RespondServiceError(w, r, err, "Unable to rotate api key")
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	if !h.requireTenantAdmin(w, r, tenant.TenantID, tenant.UserRole) {
		return
	}
	resp, err := h.Service.RevokeAPIKey(r.Context(), tenant.TenantID)
	if err != nil {
		shared.RespondServiceError(w, r, err, "Unable to revoke api key")
		return
	}
	httputil.RespondOK(w, resp)
}

func (h *Handler) requireTenantAdmin(w http.ResponseWriter, r *http.Request, tenantID int64, role string) bool {
	if tenantID == 0 {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "A tenant context is required", nil)
		return false
	}
	if role != "admin" {
		httputil.RespondErrorWithCause(w, r, http.StatusForbidden, errorcode.Forbidden, "Only tenant admins can manage this resource", nil)
		return false
	}
	return true
}

func (h *Handler) DeactivateTenant(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	if !h.requireTenantAdmin(w, r, tenant.TenantID, tenant.UserRole) {
		return
	}
	resp, err := h.Service.DeactivateTenant(r.Context(), tenant.TenantID)
	if err != nil {
		shared.RespondServiceError(w, r, err, "Unable to deactivate tenant")
		return
	}
	httputil.RespondOK(w, resp)
}

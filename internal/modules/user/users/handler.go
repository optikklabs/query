package users

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

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	if tenant.TenantID == 0 {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "A tenant context is required", nil)
		return
	}

	var req CreateUserRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "email and name are required", nil)
		return
	}

	user, err := h.Service.CreateUser(r.Context(), req, tenant.TenantID)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Failed to create user")
		return
	}
	httputil.RespondOK(w, user)
}

func (h *Handler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	if tenant.TenantID == 0 {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "A tenant context is required", nil)
		return
	}

	userID, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}

	var req UpdateRoleRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "role is required", nil)
		return
	}

	user, err := h.Service.SetUserRole(r.Context(), userID, tenant.TenantID, req.Role)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Failed to update user role")
		return
	}
	httputil.RespondOK(w, user)
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	if tenant.TenantID == 0 {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "A tenant context is required", nil)
		return
	}

	users, err := h.Service.ListUsers(r.Context(), tenant.TenantID)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Failed to list users")
		return
	}
	httputil.RespondOK(w, users)
}

func (h *Handler) RemoveUser(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	if tenant.TenantID == 0 {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "A tenant context is required", nil)
		return
	}

	userID, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := h.Service.RemoveUser(r.Context(), userID, tenant.TenantID); err != nil {
		httputil.RespondServiceError(w, r, err, "Failed to remove user")
		return
	}
	httputil.RespondOK(w, shared.MessageResponse{Message: "User deactivated"})
}

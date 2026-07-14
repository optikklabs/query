package users

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/modules/user/shared"
	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

// Handler serves the admin user-provisioning routes.
type Handler struct {
	Service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{Service: service}
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	tenant := modulecommon.Tenant(r)
	if tenant.TenantID == 0 {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "A tenant context is required", nil)
		return
	}

	var req CreateUserRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "email and name are required", nil)
		return
	}

	user, err := h.Service.CreateUser(r.Context(), req, tenant.TenantID)
	if err != nil {
		shared.RespondServiceError(w, r, err, "Failed to create user")
		return
	}
	modulecommon.RespondOK(w, user)
}

func (h *Handler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	tenant := modulecommon.Tenant(r)
	if tenant.TenantID == 0 {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "A tenant context is required", nil)
		return
	}

	userID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || userID <= 0 {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "A valid user id is required", nil)
		return
	}

	var req UpdateRoleRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "role is required", nil)
		return
	}

	user, err := h.Service.SetUserRole(userID, tenant.TenantID, req.Role)
	if err != nil {
		shared.RespondServiceError(w, r, err, "Failed to update user role")
		return
	}
	modulecommon.RespondOK(w, user)
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	tenant := modulecommon.Tenant(r)
	if tenant.TenantID == 0 {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "A tenant context is required", nil)
		return
	}

	users, err := h.Service.ListUsers(tenant.TenantID)
	if err != nil {
		shared.RespondServiceError(w, r, err, "Failed to list users")
		return
	}
	modulecommon.RespondOK(w, users)
}

func (h *Handler) RemoveUser(w http.ResponseWriter, r *http.Request) {
	tenant := modulecommon.Tenant(r)
	if tenant.TenantID == 0 {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "A tenant context is required", nil)
		return
	}

	userID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || userID <= 0 {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "A valid user id is required", nil)
		return
	}

	if err := h.Service.RemoveUser(userID, tenant.TenantID); err != nil {
		shared.RespondServiceError(w, r, err, "Failed to remove user")
		return
	}
	modulecommon.RespondOK(w, shared.MessageResponse{Message: "User deactivated"})
}

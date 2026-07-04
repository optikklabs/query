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
	var req CreateUserRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "email, name, password and tenantIds are required")
		return
	}

	user, err := h.Service.CreateUser(req)
	if err != nil {
		shared.RespondServiceError(w, r, err, "Failed to create user")
		return
	}
	modulecommon.RespondOK(w, user)
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	tenant := modulecommon.Tenant(r)
	if tenant.TenantID == 0 {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.BadRequest, "A tenant context is required")
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
	idStr := chi.URLParam(r, "id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || userID <= 0 {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "A valid user id is required")
		return
	}

	if err := h.Service.RemoveUser(userID); err != nil {
		shared.RespondServiceError(w, r, err, "Failed to remove user")
		return
	}
	modulecommon.RespondOK(w, shared.MessageResponse{Message: "User deactivated"})
}

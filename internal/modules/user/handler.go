package user

import (
	"net/http"
	"strings"

	"github.com/optikklabs/query/internal/infra/token"
	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

// Handler handles HTTP requests for users and authentication.
type Handler struct {
	Service *Service
	Tokens  *token.Service
}

func NewHandler(service *Service, tokens *token.Service) *Handler {
	return &Handler{
		Service: service,
		Tokens:  tokens,
	}
}

func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	var req CreateTeamRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "team_name and org_name are required")
		return
	}

	team, err := h.Service.CreateTeam(req)
	if err != nil {
		RespondServiceError(w, r, err, "Failed to create team")
		return
	}
	modulecommon.RespondOK(w, team)
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "email, name, password and teamIds are required")
		return
	}

	user, err := h.Service.CreateUser(req)
	if err != nil {
		RespondServiceError(w, r, err, "Failed to create user")
		return
	}
	modulecommon.RespondOK(w, user)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "Email and password are required")
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)

	response, refresh, err := h.Service.Login(r.Context(), req, modulecommon.ClientIP(r))
	if err != nil {
		RespondServiceError(w, r, err, "Failed to login")
		return
	}
	h.Tokens.SetRefreshCookie(w, refresh)
	modulecommon.RespondOK(w, response)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.Tokens.RefreshCookieName())
	if err != nil || cookie.Value == "" {
		modulecommon.RespondError(w, r, http.StatusUnauthorized, errorcode.Unauthorized, "Missing refresh token")
		return
	}

	response, refresh, err := h.Service.Refresh(r.Context(), cookie.Value)
	if err != nil {
		RespondServiceError(w, r, err, "Failed to refresh token")
		return
	}
	h.Tokens.SetRefreshCookie(w, refresh)
	modulecommon.RespondOK(w, response)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var refreshToken string
	if cookie, err := r.Cookie(h.Tokens.RefreshCookieName()); err == nil {
		refreshToken = cookie.Value
	}
	response := h.Service.Logout(r.Context(), modulecommon.Tenant(r), refreshToken, modulecommon.ClientIP(r))
	h.Tokens.ClearRefreshCookie(w)
	modulecommon.RespondOK(w, response)
}

func (h *Handler) AuthMe(w http.ResponseWriter, r *http.Request) {
	response, err := h.Service.AuthContext(modulecommon.Tenant(r).UserID)
	if err != nil {
		RespondServiceError(w, r, err, "Not authenticated")
		return
	}
	modulecommon.RespondOK(w, response)
}

func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := h.Service.GetProfile(modulecommon.Tenant(r).UserID)
	if err != nil {
		RespondServiceError(w, r, err, "User not found")
		return
	}
	modulecommon.RespondOK(w, profile)
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req UpdateProfileRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body")
		return
	}

	profile, err := h.Service.UpdateProfile(modulecommon.Tenant(r).UserID, req)
	if err != nil {
		RespondServiceError(w, r, err, "Unable to update profile")
		return
	}
	modulecommon.RespondOK(w, profile)
}

func (h *Handler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	var req UpdatePreferencesRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body")
		return
	}

	response, err := h.Service.UpdatePreferences(modulecommon.Tenant(r).UserID, req)
	if err != nil {
		RespondServiceError(w, r, err, "Unable to update preferences")
		return
	}

	modulecommon.RespondOK(w, response)
}

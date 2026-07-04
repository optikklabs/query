package user

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/infra/token"
	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

// Handler handles HTTP requests for users and authentication.
type Handler struct {
	Service       *Service
	Tokens        *token.Service
	signupLimiter *signupLimiter
}

func NewHandler(service *Service, tokens *token.Service) *Handler {
	return &Handler{
		Service:       service,
		Tokens:        tokens,
		signupLimiter: newSignupLimiter(5, time.Hour),
	}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	if !h.signupLimiter.Allow(modulecommon.ClientIP(r)) {
		modulecommon.RespondError(w, r, http.StatusTooManyRequests, errorcode.RateLimited, "Too many signup attempts, please try again later")
		return
	}

	var req SignupRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "email, password, name and org_name are required")
		return
	}

	response, refresh, err := h.Service.Signup(r.Context(), req, modulecommon.ClientIP(r))
	if err != nil {
		RespondServiceError(w, r, err, "Failed to sign up")
		return
	}
	h.Tokens.SetRefreshCookie(w, refresh)
	modulecommon.RespondOK(w, response)
}

func (h *Handler) DeviceCode(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Service.StartDeviceAuth(r.Context())
	if err != nil {
		RespondServiceError(w, r, err, "Failed to start device authorization")
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) DeviceToken(w http.ResponseWriter, r *http.Request) {
	var req DeviceTokenRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "device_code is required")
		return
	}

	session, refresh, err := h.Service.PollDeviceToken(r.Context(), strings.TrimSpace(req.DeviceCode))
	switch {
	case err == nil:
		h.Tokens.SetRefreshCookie(w, refresh)
		modulecommon.RespondOK(w, DeviceTokenResponse{Status: "complete", Session: &session})
	case errors.Is(err, ErrDeviceAuthPending):
		modulecommon.RespondOK(w, DeviceTokenResponse{Status: "authorization_pending"})
	case errors.Is(err, ErrDeviceSlowDown):
		modulecommon.RespondOK(w, DeviceTokenResponse{Status: "slow_down"})
	case errors.Is(err, ErrDeviceExpired):
		modulecommon.RespondOK(w, DeviceTokenResponse{Status: "expired_token"})
	default:
		RespondServiceError(w, r, err, "Failed to poll device token")
	}
}

func (h *Handler) DeviceApprove(w http.ResponseWriter, r *http.Request) {
	var req DeviceApproveRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "user_code is required")
		return
	}

	userCode := strings.ToUpper(strings.TrimSpace(req.UserCode))
	if err := h.Service.ApproveDeviceCode(r.Context(), userCode, modulecommon.Tenant(r).UserID); err != nil {
		RespondServiceError(w, r, err, "Failed to approve device")
		return
	}
	modulecommon.RespondOK(w, MessageResponse{Message: "Device approved. Return to your terminal."})
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

func (h *Handler) RotateAPIKey(w http.ResponseWriter, r *http.Request) {
	tenant := modulecommon.Tenant(r)
	if !h.requireTeamAdmin(w, r, tenant.TeamID, tenant.UserRole) {
		return
	}
	team, err := h.Service.RotateAPIKey(r.Context(), tenant.TeamID)
	if err != nil {
		RespondServiceError(w, r, err, "Unable to rotate api key")
		return
	}
	modulecommon.RespondOK(w, team)
}

func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	tenant := modulecommon.Tenant(r)
	if !h.requireTeamAdmin(w, r, tenant.TeamID, tenant.UserRole) {
		return
	}
	team, err := h.Service.RevokeAPIKey(r.Context(), tenant.TeamID)
	if err != nil {
		RespondServiceError(w, r, err, "Unable to revoke api key")
		return
	}
	modulecommon.RespondOK(w, team)
}

// requireTeamAdmin gates key management to a team's admins.
func (h *Handler) requireTeamAdmin(w http.ResponseWriter, r *http.Request, teamID int64, role string) bool {
	if teamID == 0 {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.BadRequest, "A team context is required")
		return false
	}
	if role != "admin" {
		modulecommon.RespondError(w, r, http.StatusForbidden, errorcode.Forbidden, "Only team admins can manage API keys")
		return false
	}
	return true
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

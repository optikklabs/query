package auth

import (
	"log/slog"
	"net/http"

	"github.com/optikklabs/query/internal/infra/token"
	"github.com/optikklabs/query/internal/modules/user/shared"
	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

// Handler serves the auth routes (login, refresh, logout, signup).
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

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Email and password are required", nil)
		return
	}

	response, refresh, err := h.Service.Login(r.Context(), req, modulecommon.ClientIP(r))
	if err != nil {
		shared.RespondServiceError(w, r, err, "Failed to login")
		return
	}
	h.Tokens.SetRefreshCookie(w, refresh)
	modulecommon.RespondOK(w, response)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	candidates := h.Tokens.RefreshCookieValues(r)
	if len(candidates) == 0 {
		// The browser sent no refresh cookie at all — the most common cause of
		// a forced re-login, so record which client hit it.
		slog.WarnContext(r.Context(), "AUTH_EVENT refresh_no_cookie",
			slog.String("ip", modulecommon.ClientIP(r)),
			slog.String("user_agent", r.UserAgent()))
		modulecommon.RespondErrorWithCause(w, r, http.StatusUnauthorized, errorcode.Unauthorized, "Missing refresh token", nil)
		return
	}

	response, refresh, err := h.Service.Refresh(r.Context(), candidates, modulecommon.ClientIP(r))
	if err != nil {
		shared.RespondServiceError(w, r, err, "Failed to refresh token")
		return
	}
	h.Tokens.SetRefreshCookie(w, refresh)
	modulecommon.RespondOK(w, response)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	response := h.Service.Logout(r.Context(), modulecommon.Tenant(r), h.Tokens.RefreshCookieValues(r), modulecommon.ClientIP(r))
	h.Tokens.ClearRefreshCookie(w)
	modulecommon.RespondOK(w, response)
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Email is required", nil)
		return
	}
	if err := h.Service.ForgotPassword(r.Context(), req.Email); err != nil {
		shared.RespondServiceError(w, r, err, "Failed to process forgot password request")
		return
	}
	modulecommon.RespondOK(w, shared.MessageResponse{Message: "If your email is registered, a password reset link has been sent."})
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Token and new password are required", nil)
		return
	}
	if err := h.Service.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		shared.RespondServiceError(w, r, err, "Failed to reset password")
		return
	}
	modulecommon.RespondOK(w, shared.MessageResponse{Message: "Password has been successfully reset."})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	tenant := modulecommon.Tenant(r)
	if tenant.UserID <= 0 {
		modulecommon.RespondErrorWithCause(w, r, http.StatusUnauthorized, errorcode.Unauthorized, "Authentication required", nil)
		return
	}

	var req ChangePasswordRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Current and new password are required", nil)
		return
	}
	if err := h.Service.ChangePassword(r.Context(), tenant.UserID, req.CurrentPassword, req.NewPassword); err != nil {
		shared.RespondServiceError(w, r, err, "Failed to change password")
		return
	}
	modulecommon.RespondOK(w, shared.MessageResponse{Message: "Password changed successfully."})
}

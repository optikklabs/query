package auth

import (
	"net/http"
	"strings"

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

	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)

	response, refresh, err := h.Service.Login(r.Context(), req, modulecommon.ClientIP(r))
	if err != nil {
		shared.RespondServiceError(w, r, err, "Failed to login")
		return
	}
	h.Tokens.SetRefreshCookie(w, refresh)
	modulecommon.RespondOK(w, response)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.Tokens.RefreshCookieName())
	if err != nil || cookie.Value == "" {
		modulecommon.RespondErrorWithCause(w, r, http.StatusUnauthorized, errorcode.Unauthorized, "Missing refresh token", nil)
		return
	}

	response, refresh, err := h.Service.Refresh(r.Context(), cookie.Value)
	if err != nil {
		shared.RespondServiceError(w, r, err, "Failed to refresh token")
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


package signup

import (
	"net/http"

	"github.com/optikklabs/query/internal/infra/token"
	"github.com/optikklabs/query/internal/modules/user/auth"
	"github.com/optikklabs/query/internal/modules/user/shared"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
	Tokens  *token.Service
}

func NewHandler(service *Service, tokens *token.Service) *Handler {
	return &Handler{Service: service, Tokens: tokens}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid signup request", nil)
		return
	}

	response, err := h.Service.Signup(r.Context(), req)
	if err != nil {
		shared.RespondServiceError(w, r, err, "Failed to create account")
		return
	}
	if response.Session != nil {
		h.Tokens.SetRefreshCookie(w, response.RefreshToken)
		httputil.RespondOK(w, struct {
			auth.LoginResponse
			APIKey string `json:"apiKey"`
		}{*response.Session, response.APIKey})
		return
	}
	httputil.RespondOK(w, SignupResponse{Message: response.Message})
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid verification request", nil)
		return
	}
	response, refresh, apiKey, err := h.Service.VerifyEmail(r.Context(), req.Token)
	if err != nil {
		shared.RespondServiceError(w, r, err, "Unable to verify email")
		return
	}
	h.Tokens.SetRefreshCookie(w, refresh)
	httputil.RespondOK(w, struct {
		auth.LoginResponse
		APIKey string `json:"apiKey"`
	}{response, apiKey})
}

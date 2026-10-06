package signup

import (
	"net/http"

	"github.com/optikklabs/query/internal/infra/token"
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
	if !httputil.BindJSON(w, r, &req) {
		return
	}

	response, err := h.Service.Signup(r.Context(), req)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Failed to create account")
		return
	}
	if response.Session != nil {
		h.Tokens.SetRefreshCookie(w, response.RefreshToken)
		httputil.RespondOK(w, SessionResponse{LoginResponse: *response.Session, APIKey: response.APIKey})
		return
	}
	httputil.RespondOK(w, SignupResponse{Message: response.Message})
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	response, refresh, apiKey, err := h.Service.VerifyEmail(r.Context(), req.Token)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "Unable to verify email")
		return
	}
	h.Tokens.SetRefreshCookie(w, refresh)
	httputil.RespondOK(w, SessionResponse{LoginResponse: response, APIKey: apiKey})
}

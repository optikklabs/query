package signup

import (
	"net/http"

	"github.com/optikklabs/query/internal/infra/token"
	"github.com/optikklabs/query/internal/modules/user/auth"
	"github.com/optikklabs/query/internal/modules/user/shared"
	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

// Handler serves the public signup route.
type Handler struct {
	Service *Service
	Tokens  *token.Service
}

func NewHandler(service *Service, tokens *token.Service) *Handler {
	return &Handler{Service: service, Tokens: tokens}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid signup request")
		return
	}

	response, err := h.Service.Signup(r.Context(), req, modulecommon.ClientIP(r))
	if err != nil {
		shared.RespondServiceError(w, r, err, "Failed to create account")
		return
	}
	modulecommon.RespondOK(w, response)
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if err := modulecommon.DecodeJSON(r, &req); err != nil {
		modulecommon.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid verification request")
		return
	}
	response, refresh, apiKey, err := h.Service.VerifyEmail(r.Context(), req.Token)
	if err != nil {
		shared.RespondServiceError(w, r, err, "Unable to verify email")
		return
	}
	h.Tokens.SetRefreshCookie(w, refresh)
	modulecommon.RespondOK(w, struct {
		auth.LoginResponse
		APIKey string `json:"api_key"`
	}{response, apiKey})
}

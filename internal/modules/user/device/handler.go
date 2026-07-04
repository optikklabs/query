package device

import (
	"errors"
	"net/http"
	"strings"

	"github.com/optikklabs/query/internal/infra/token"
	"github.com/optikklabs/query/internal/modules/user/shared"
	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

// Handler serves the device-authorization routes.
type Handler struct {
	Service *Service
	Tokens  *token.Service
}

func NewHandler(service *Service, tokens *token.Service) *Handler {
	return &Handler{Service: service, Tokens: tokens}
}

func (h *Handler) DeviceCode(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Service.StartDeviceAuth(r.Context())
	if err != nil {
		shared.RespondServiceError(w, r, err, "Failed to start device authorization")
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
		shared.RespondServiceError(w, r, err, "Failed to poll device token")
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
		shared.RespondServiceError(w, r, err, "Failed to approve device")
		return
	}
	modulecommon.RespondOK(w, shared.MessageResponse{Message: "Device approved. Return to your terminal."})
}

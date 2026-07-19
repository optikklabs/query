package device

import "github.com/optikklabs/query/internal/modules/user/auth"

type DeviceCodeResponse struct {
	DeviceCode string `json:"deviceCode"`
	UserCode   string `json:"userCode"`
	Interval   int    `json:"interval"`
	ExpiresIn  int    `json:"expiresIn"`
}

type DeviceTokenRequest struct {
	DeviceCode string `json:"deviceCode" validate:"required"`
}

// DeviceTokenResponse always returns 200; the CLI switches on Status
// (authorization_pending|slow_down|expired_token|complete). Session is set
// only when complete.
type DeviceTokenResponse struct {
	Status  string              `json:"status"`
	Session *auth.LoginResponse `json:"session,omitempty"`
}

type DeviceApproveRequest struct {
	UserCode string `json:"userCode" validate:"required"`
}

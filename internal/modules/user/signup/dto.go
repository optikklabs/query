package signup

import "github.com/optikklabs/query/internal/modules/user/auth"

type SignupRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	Name          string `json:"name"`
	TenantName    string `json:"tenantName"`
	AcceptedTerms bool   `json:"acceptedTerms"`
}

// SessionResponse is a new session plus the tenant's freshly issued API key,
// shown once.
type SessionResponse struct {
	auth.LoginResponse
	APIKey string `json:"apiKey"`
}

type SignupResponse struct {
	Message string `json:"message"`
}

type VerifyEmailRequest struct {
	Token string `json:"token"`
}

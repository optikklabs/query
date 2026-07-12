package signup

// SignupRequest self-serves an account plus its first tenant. tenant_name is
// snake_case to match the web and CLI clients that post this body.
type SignupRequest struct {
	Email      string `json:"email" validate:"required,email"`
	Password   string `json:"password" validate:"required,min=8"`
	Name       string `json:"name" validate:"required"`
	TenantName string `json:"tenant_name" validate:"required"`
}

// SignupResponse embeds the standard auth session and adds the tenant api_key.
// Only signup and settings-rotate ever expose api_key; login/device never do.
type SignupResponse struct {
	Message string `json:"message"`
}

type VerifyEmailRequest struct {
	Token string `json:"token" validate:"required,len=64"`
}

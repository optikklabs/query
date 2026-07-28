package signup

type SignupRequest struct {
	Email         string `json:"email" validate:"required,email"`
	Password      string `json:"password" validate:"required,min=8"`
	Name          string `json:"name" validate:"required"`
	TenantName    string `json:"tenantName" validate:"required"`
	AcceptedTerms bool   `json:"acceptedTerms" validate:"eq=true"`
}

type SignupResponse struct {
	Message string `json:"message"`
}

type VerifyEmailRequest struct {
	Token string `json:"token" validate:"required,len=64"`
}

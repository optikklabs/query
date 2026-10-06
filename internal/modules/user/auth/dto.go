package auth

import "time"

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AuthContextResponse
	AccessToken string `json:"accessToken"`
}

type AuthUserSummary struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type AuthTenantSummary struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Role          string     `json:"role"`
	AccountStatus string     `json:"accountStatus"`
	TrialEndsAt   *time.Time `json:"trialEndsAt,omitempty"`
}

type AuthContextResponse struct {
	User   AuthUserSummary   `json:"user"`
	Tenant AuthTenantSummary `json:"tenant"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

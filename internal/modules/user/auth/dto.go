package auth

import "time"

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"user@example.com"`
	Password string `json:"password" validate:"required" example:"securePassword123"`
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
	Email string `json:"email" validate:"required,email" example:"user@example.com"`
}

type ResetPasswordRequest struct {
	Token    string `json:"token" validate:"required"`
	Password string `json:"password" validate:"required,min=8" example:"securePassword123"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required,min=8" example:"securePassword123"`
}

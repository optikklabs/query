package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/config"
	"github.com/optikklabs/query/internal/infra/token"
	"github.com/optikklabs/query/internal/modules/user/shared"
	contracts "github.com/optikklabs/query/internal/shared/contracts"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

type Service struct {
	repo     *Repository
	tokens   *token.Service
	attempts *loginAttempts
	sender   PasswordResetSender
}

func NewService(repo *Repository, tokens *token.Service, emailCfg config.EmailConfig) *Service {
	sender := PasswordResetSender(noopPasswordResetSender{})
	if emailCfg.ResendVerificationEnabled {
		sender = NewResendPasswordResetSender(emailCfg.ResendAPIKey, emailCfg.From, emailCfg.VerifyBaseURL+"/reset-password")
	}
	return &Service{
		repo:     repo,
		tokens:   tokens,
		attempts: &loginAttempts{entries: make(map[string]attempt)},
		sender:   sender,
	}
}

func (s *Service) Login(ctx context.Context, req LoginRequest, clientIP string) (LoginResponse, string, error) {
	email := strings.TrimSpace(req.Email)
	if !s.attempts.allow(email, clientIP) {
		return LoginResponse{}, "", errorcode.ValidationError{Msg: "Too many login attempts. Try again later."}
	}

	user, err := s.repo.FindActiveUserByEmail(ctx, email)
	if err != nil {
		s.attempts.fail(email, clientIP)
		return LoginResponse{}, "", errorcode.ValidationError{Msg: "Invalid email or password"}
	}

	if !shared.PasswordIsValid(user.PasswordHash, req.Password) {
		s.attempts.fail(email, clientIP)
		return LoginResponse{}, "", errorcode.ValidationError{Msg: "Invalid email or password"}
	}
	s.attempts.reset(email, clientIP)

	response, refresh, err := s.issueTokens(ctx, user, token.NewFamilyID())
	if err != nil {
		return LoginResponse{}, "", err
	}

	slog.InfoContext(ctx, "AUTH_EVENT login_success", slog.Int64("user_id", user.ID), slog.String("email", user.Email), slog.String("ip", clientIP))
	return response, refresh, nil
}

func (s *Service) IssueTokens(ctx context.Context, user shared.AuthUser) (LoginResponse, string, error) {
	return s.issueTokens(ctx, user, token.NewFamilyID())
}

func (s *Service) issueTokens(ctx context.Context, user shared.AuthUser, familyID string) (LoginResponse, string, error) {
	response, err := s.buildAuthContextResponse(ctx, user)
	if err != nil {
		return LoginResponse{}, "", err
	}

	access, err := s.signAccess(user, response.Tenant.ID)
	if err != nil {
		return LoginResponse{}, "", err
	}

	raw, hash, err := token.GenerateRefreshToken()
	if err != nil {
		return LoginResponse{}, "", fmt.Errorf("failed to issue refresh token: %w", err)
	}
	expiresAt := time.Now().UTC().Add(s.tokens.RefreshTTL())
	if err := s.repo.InsertRefreshToken(ctx, user.ID, familyID, hash, expiresAt); err != nil {
		return LoginResponse{}, "", fmt.Errorf("failed to issue refresh token: %w", err)
	}

	return LoginResponse{AuthContextResponse: response, AccessToken: access}, raw, nil
}

func (s *Service) signAccess(user shared.AuthUser, tenantID int64) (string, error) {
	access, err := s.tokens.SignAccess(token.AuthState{
		UserID:          user.ID,
		Email:           user.Email,
		Role:            user.Role,
		DefaultTenantID: tenantID,
		TenantIDs:       []int64{tenantID},
	})
	if err != nil {
		return "", fmt.Errorf("failed to issue access token: %w", err)
	}
	return access, nil
}

func (s *Service) Logout(ctx context.Context, tenant contracts.TenantContext, refreshTokens []string, clientIP string) shared.MessageResponse {

	for _, refreshToken := range refreshTokens {
		if refreshToken == "" {
			continue
		}
		if err := s.repo.RevokeRefreshToken(ctx, token.HashRefreshToken(refreshToken)); err != nil {
			slog.WarnContext(ctx, "AUTH_EVENT logout_revoke_failed", slog.Int64("user_id", tenant.UserID), slog.Any("error", err))
		}
	}
	if tenant.UserID > 0 {
		slog.InfoContext(ctx, "AUTH_EVENT logout", slog.Int64("user_id", tenant.UserID), slog.String("email", tenant.UserEmail), slog.String("ip", clientIP))
	}
	return shared.MessageResponse{Message: "Logged out successfully"}
}

func (s *Service) buildAuthContextResponse(ctx context.Context, user shared.AuthUser) (AuthContextResponse, error) {
	tenant, err := s.tenantForUser(ctx, user.TenantID)
	if err != nil {
		slog.Warn("AUTH_EVENT tenant_fetch_failed", slog.Int64("user_id", user.ID), slog.String("email", user.Email), slog.Any("error", err))

		return AuthContextResponse{}, err
	}

	tenant.Role = user.Role

	return AuthContextResponse{
		User: AuthUserSummary{
			ID:    user.ID,
			Email: user.Email,
			Name:  user.Name,
		},
		Tenant: tenant,
	}, nil
}

func (s *Service) tenantForUser(ctx context.Context, tenantID int64) (AuthTenantSummary, error) {
	if tenantID <= 0 {
		return AuthTenantSummary{}, errorcode.ValidationError{Msg: "Account has no associated tenant"}
	}

	tenant, err := s.repo.FindTenantByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AuthTenantSummary{}, errorcode.ValidationError{Msg: "Account has no active tenant"}
		}
		return AuthTenantSummary{}, err
	}
	if !tenant.Active {
		if tenant.AccountStatus == "suspended" {
			return AuthTenantSummary{}, shared.TrialExpiredError{Msg: "Your free trial has ended. Add a payment method to resume."}
		}
		return AuthTenantSummary{}, errorcode.ValidationError{Msg: "Account has no active tenant"}
	}

	return AuthTenantSummary{
		ID:   tenant.ID,
		Name: tenant.Name,

		AccountStatus: tenant.AccountStatus,
		TrialEndsAt:   tenant.TrialEndsAt,
	}, nil
}

func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return errorcode.ValidationError{Msg: "Email is required"}
	}

	user, err := s.repo.FindActiveUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {

			return nil
		}
		return fmt.Errorf("failed to lookup user: %w", err)
	}

	hash := ""
	if user.PasswordHash != nil {
		hash = *user.PasswordHash
	}

	resetToken, err := s.tokens.SignPasswordReset(user.ID, hash)
	if err != nil {
		return fmt.Errorf("failed to generate reset token: %w", err)
	}

	if err := s.sender.SendPasswordReset(ctx, user.Email, resetToken); err != nil {
		return fmt.Errorf("failed to send password reset email: %w", err)
	}

	slog.InfoContext(ctx, "AUTH_EVENT forgot_password_requested", slog.Int64("user_id", user.ID))
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, tokenStr string, newPassword string) error {
	if len(newPassword) < shared.MinPasswordLength {
		return errorcode.ValidationError{Msg: "Password must be at least 8 characters"}
	}

	userID, err := s.tokens.ExtractSubjectWithoutVerify(tokenStr)
	if err != nil {
		return shared.UnauthorizedError{Msg: "Invalid reset token"}
	}

	user, err := s.repo.FindAuthUserByID(ctx, userID)
	if err != nil {
		return shared.UnauthorizedError{Msg: "Invalid reset token"}
	}

	hash := ""
	if user.PasswordHash != nil {
		hash = *user.PasswordHash
	}

	verifiedUserID, err := s.tokens.ParsePasswordReset(tokenStr, hash)
	if err != nil || verifiedUserID != userID {
		return shared.UnauthorizedError{Msg: "Invalid or expired reset token"}
	}

	newHash, err := shared.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.repo.UpdatePasswordAndRevokeSessions(ctx, userID, newHash); err != nil {
		return fmt.Errorf("failed to update password and revoke existing sessions: %w", err)
	}

	slog.InfoContext(ctx, "AUTH_EVENT password_reset_success", slog.Int64("user_id", userID))
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error {
	if len(newPassword) < shared.MinPasswordLength {
		return errorcode.ValidationError{Msg: "New password must be at least 8 characters"}
	}

	user, err := s.repo.FindAuthUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to lookup user: %w", err)
	}

	if !shared.PasswordIsValid(user.PasswordHash, currentPassword) {
		return errorcode.ValidationError{Msg: "Invalid current password"}
	}

	newHash, err := shared.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.repo.UpdatePasswordAndRevokeSessions(ctx, userID, newHash); err != nil {
		return fmt.Errorf("failed to update password and revoke existing sessions: %w", err)
	}

	slog.InfoContext(ctx, "AUTH_EVENT password_changed", slog.Int64("user_id", userID))
	return nil
}

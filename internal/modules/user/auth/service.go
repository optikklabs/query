package auth

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/infra/token"
	"github.com/optikklabs/query/internal/modules/user/shared"
	contracts "github.com/optikklabs/query/internal/shared/contracts"
	"golang.org/x/crypto/bcrypt"
)

// Service handles authentication and session issuance.
type Service struct {
	repo   *Repository
	tokens *token.Service
}

func NewService(repo *Repository, tokens *token.Service) *Service {
	return &Service{repo: repo, tokens: tokens}
}

// Login authenticates a user and issues access and refresh tokens.
func (s *Service) Login(ctx context.Context, req LoginRequest, clientIP string) (LoginResponse, string, error) {
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)

	user, err := s.repo.FindActiveUserByEmail(email)
	if err != nil {
		return LoginResponse{}, "", shared.NewValidationError("Invalid email or password", err)
	}

	if user.PasswordHash != nil && *user.PasswordHash != "" && bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)) != nil {
		return LoginResponse{}, "", shared.NewValidationError("Invalid email or password", nil)
	}



	response, refresh, err := s.issueTokens(user, token.NewFamilyID())
	if err != nil {
		return LoginResponse{}, "", err
	}

	slog.InfoContext(ctx, "AUTH_EVENT login_success", slog.Int64("user_id", user.ID), slog.String("email", user.Email), slog.String("ip", clientIP))
	return response, refresh, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (LoginResponse, string, error) {
	hash := token.HashRefreshToken(refreshToken)
	stored, err := s.repo.FindRefreshTokenByHash(hash)
	if err != nil {
		return LoginResponse{}, "", shared.NewUnauthorizedError("Invalid or expired refresh token", err)
	}

	if stored.RevokedAt != nil {
		_ = s.repo.RevokeRefreshTokenFamily(stored.FamilyID)
		slog.WarnContext(ctx, "AUTH_EVENT refresh_reuse_detected", slog.Int64("user_id", stored.UserID), slog.String("family_id", stored.FamilyID))
		return LoginResponse{}, "", shared.NewUnauthorizedError("Invalid or expired refresh token", nil)
	}

	if time.Now().UTC().After(stored.ExpiresAt) {
		return LoginResponse{}, "", shared.NewUnauthorizedError("Invalid or expired refresh token", nil)
	}

	user, err := s.repo.FindActiveUserByID(stored.UserID)
	if err != nil {
		return LoginResponse{}, "", shared.NewUnauthorizedError("Invalid or expired refresh token", err)
	}

	if err := s.repo.RevokeRefreshToken(hash); err != nil {
		return LoginResponse{}, "", shared.NewInternalError("Failed to rotate refresh token", err)
	}

	authUser := shared.AuthUser{
		ID:       user.ID,
		Email:    user.Email,
		Name:     user.Name,
		TenantID: user.TenantID,
		Role:     user.Role,
	}
	response, refresh, err := s.issueTokens(authUser, stored.FamilyID)
	if err != nil {
		return LoginResponse{}, "", err
	}
	return response, refresh, nil
}

// IssueTokens mints a fresh session (new token family) for a user. Used by the
// onboarding (signup) and device flows to complete login after they identify the
// user; refresh-token and tenant reads stay owned by auth.
func (s *Service) IssueTokens(user shared.AuthUser) (LoginResponse, string, error) {
	return s.issueTokens(user, token.NewFamilyID())
}

func (s *Service) issueTokens(user shared.AuthUser, familyID string) (LoginResponse, string, error) {
	response, err := s.buildAuthContextResponse(user)
	if err != nil {
		return LoginResponse{}, "", err
	}

	access, err := s.tokens.SignAccess(token.AuthState{
		UserID:          user.ID,
		Email:           user.Email,
		Role:            user.Role,
		DefaultTenantID: response.Tenant.ID,
		TenantIDs:       []int64{response.Tenant.ID},
	})
	if err != nil {
		return LoginResponse{}, "", shared.NewInternalError("Failed to issue access token", err)
	}

	raw, hash, err := token.GenerateRefreshToken()
	if err != nil {
		return LoginResponse{}, "", shared.NewInternalError("Failed to issue refresh token", err)
	}
	expiresAt := time.Now().UTC().Add(s.tokens.RefreshTTL())
	if err := s.repo.InsertRefreshToken(user.ID, familyID, hash, expiresAt); err != nil {
		return LoginResponse{}, "", shared.NewInternalError("Failed to issue refresh token", err)
	}

	return LoginResponse{AuthContextResponse: response, AccessToken: access}, raw, nil
}

func (s *Service) Logout(ctx context.Context, tenant contracts.TenantContext, refreshToken, clientIP string) shared.MessageResponse {
	if refreshToken != "" {
		if err := s.repo.RevokeRefreshToken(token.HashRefreshToken(refreshToken)); err != nil {
			slog.WarnContext(ctx, "AUTH_EVENT logout_revoke_failed", slog.Int64("user_id", tenant.UserID), slog.Any("error", err))
		}
	}
	if tenant.UserID > 0 {
		slog.InfoContext(ctx, "AUTH_EVENT logout", slog.Int64("user_id", tenant.UserID), slog.String("email", tenant.UserEmail), slog.String("ip", clientIP))
	}
	return shared.MessageResponse{Message: "Logged out successfully"}
}

func (s *Service) buildAuthContextResponse(user shared.AuthUser) (AuthContextResponse, error) {
	tenant, err := s.tenantForUser(user.TenantID)
	if err != nil {
		slog.Warn("AUTH_EVENT tenant_fetch_failed", slog.Int64("user_id", user.ID), slog.String("email", user.Email), slog.Any("error", err))
		// Propagate the typed error (e.g. TRIAL_EXPIRED) so callers can react.
		return AuthContextResponse{}, err
	}
	// Role is a property of the user within their tenant, not of the tenant.
	tenant.Role = user.Role

	return AuthContextResponse{
		User: AuthUserSummary{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
		},
		Tenant: tenant,
	}, nil
}

func (s *Service) tenantForUser(tenantID int64) (AuthTenantSummary, error) {
	if tenantID <= 0 {
		return AuthTenantSummary{}, shared.NewValidationError("Account has no associated tenant", nil)
	}

	tenant, err := s.repo.FindTenantByID(tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AuthTenantSummary{}, shared.NewValidationError("Account has no active tenant", nil)
		}
		return AuthTenantSummary{}, err
	}
	if !tenant.Active {
		if tenant.AccountStatus == "suspended" {
			return AuthTenantSummary{}, shared.NewTrialExpiredError(
				"Your free trial has ended. Add a payment method to resume.", nil)
		}
		return AuthTenantSummary{}, shared.NewValidationError("Account has no active tenant", nil)
	}

	return AuthTenantSummary{
		ID:            tenant.ID,
		Name:          tenant.Name,
		// Role is set by the caller from the user's own role.
		AccountStatus: tenant.AccountStatus,
		TrialEndsAt:   tenant.TrialEndsAt,
	}, nil
}


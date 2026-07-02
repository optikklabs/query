package user

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/infra/token"
	contracts "github.com/optikklabs/query/internal/shared/contracts"
	"golang.org/x/crypto/bcrypt"
)

// Login authenticates a user and issues access and refresh tokens.
func (s *Service) Login(ctx context.Context, req LoginRequest, clientIP string) (LoginResponse, string, error) {
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)

	user, err := s.repo.FindActiveUserByEmail(email)
	if err != nil {
		return LoginResponse{}, "", NewValidationError("Invalid email or password", err)
	}

	if user.PasswordHash != nil && *user.PasswordHash != "" && bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)) != nil {
		return LoginResponse{}, "", NewValidationError("Invalid email or password", nil)
	}

	if err := s.repo.UpdateUserLastLogin(user.ID, time.Now().UTC()); err != nil {
		slog.WarnContext(ctx, "AUTH_EVENT login_update_failed", slog.Int64("user_id", user.ID), slog.String("email", user.Email), slog.Any("error", err))
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
		return LoginResponse{}, "", NewUnauthorizedError("Invalid or expired refresh token", err)
	}

	if stored.RevokedAt != nil {
		_ = s.repo.RevokeRefreshTokenFamily(stored.FamilyID)
		slog.WarnContext(ctx, "AUTH_EVENT refresh_reuse_detected", slog.Int64("user_id", stored.UserID), slog.String("family_id", stored.FamilyID))
		return LoginResponse{}, "", NewUnauthorizedError("Invalid or expired refresh token", nil)
	}

	if time.Now().UTC().After(stored.ExpiresAt) {
		return LoginResponse{}, "", NewUnauthorizedError("Invalid or expired refresh token", nil)
	}

	user, err := s.repo.FindActiveUserByID(stored.UserID)
	if err != nil {
		return LoginResponse{}, "", NewUnauthorizedError("Invalid or expired refresh token", err)
	}

	if err := s.repo.RevokeRefreshToken(hash); err != nil {
		return LoginResponse{}, "", NewInternalError("Failed to rotate refresh token", err)
	}

	authUser := AuthUser{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		AvatarURL: user.AvatarURL,
		TeamsJSON: user.TeamsJSON,
	}
	response, refresh, err := s.issueTokens(authUser, stored.FamilyID)
	if err != nil {
		return LoginResponse{}, "", err
	}
	return response, refresh, nil
}

func (s *Service) issueTokens(user AuthUser, familyID string) (LoginResponse, string, error) {
	response, err := s.buildAuthContextResponse(user)
	if err != nil {
		return LoginResponse{}, "", err
	}

	access, err := s.tokens.SignAccess(token.AuthState{
		UserID:        user.ID,
		Email:         user.Email,
		Role:          response.Team.Role,
		IsAdmin:       user.IsAdmin,
		DefaultTeamID: response.Team.ID,
		TeamIDs:       []int64{response.Team.ID},
	})
	if err != nil {
		return LoginResponse{}, "", NewInternalError("Failed to issue access token", err)
	}

	raw, hash, err := token.GenerateRefreshToken()
	if err != nil {
		return LoginResponse{}, "", NewInternalError("Failed to issue refresh token", err)
	}
	expiresAt := time.Now().UTC().Add(s.tokens.RefreshTTL())
	if err := s.repo.InsertRefreshToken(user.ID, familyID, hash, expiresAt); err != nil {
		return LoginResponse{}, "", NewInternalError("Failed to issue refresh token", err)
	}

	return LoginResponse{AuthContextResponse: response, AccessToken: access}, raw, nil
}

func (s *Service) Logout(ctx context.Context, tenant contracts.TenantContext, refreshToken, clientIP string) MessageResponse {
	if refreshToken != "" {
		if err := s.repo.RevokeRefreshToken(token.HashRefreshToken(refreshToken)); err != nil {
			slog.WarnContext(ctx, "AUTH_EVENT logout_revoke_failed", slog.Int64("user_id", tenant.UserID), slog.Any("error", err))
		}
	}
	if tenant.UserID > 0 {
		slog.InfoContext(ctx, "AUTH_EVENT logout", slog.Int64("user_id", tenant.UserID), slog.String("email", tenant.UserEmail), slog.String("ip", clientIP))
	}
	return MessageResponse{Message: "Logged out successfully"}
}


func (s *Service) ValidateToken(tenant contracts.TenantContext) (ValidateTokenResponse, error) {
	if tenant.UserID == 0 {
		return ValidateTokenResponse{}, NewUnauthorizedError("Invalid or expired session", nil)
	}
	return ValidateTokenResponse{
		Valid:  true,
		UserID: tenant.UserID,
		TeamID: tenant.TeamID,
		Role:   tenant.UserRole,
	}, nil
}

func (s *Service) ForgotPassword() MessageResponse {
	return MessageResponse{
		Message: "Password resets are managed by your IT administrator. Please contact your IT admin for assistance.",
	}
}

func (s *Service) buildAuthContextResponse(user AuthUser) (AuthContextResponse, error) {
	team, err := s.teamForUser(user.TeamsJSON)
	if err != nil {
		// Platform super-admins exist before any team; let them log in team-less
		// so they can provision the first tenant.
		if user.IsAdmin {
			return AuthContextResponse{User: AuthUserSummary{ID: user.ID, Email: user.Email, Name: user.Name, AvatarURL: user.AvatarURL}}, nil
		}
		slog.Warn("AUTH_EVENT team_fetch_failed", slog.Int64("user_id", user.ID), slog.String("email", user.Email), slog.Any("error", err))
		return AuthContextResponse{}, NewValidationError("Account has no associated team. Contact your administrator.", err)
	}

	return AuthContextResponse{
		User: AuthUserSummary{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			AvatarURL: user.AvatarURL,
		},
		Team: team,
	}, nil
}

func (s *Service) teamForUser(teamsJSON *string) (AuthTeamSummary, error) {
	memberships, err := ParseTeamMemberships(ValueOr(teamsJSON, "[]"))
	if err != nil {
		return AuthTeamSummary{}, err
	}
	if len(memberships) == 0 {
		return AuthTeamSummary{}, NewValidationError("Account has no associated team", nil)
	}

	membership := memberships[0]
	teams, err := s.repo.ListActiveTeamsByIDs([]int64{membership.TeamID})
	if err != nil {
		return AuthTeamSummary{}, err
	}
	if len(teams) == 0 {
		return AuthTeamSummary{}, NewValidationError("Account has no active team", nil)
	}

	team := teams[0]
	return AuthTeamSummary{
		ID:      team.ID,
		Name:    team.Name,
		Slug:    team.Slug,
		Color:   team.Color,
		OrgName: team.OrgName,
		Role:    membership.Role,
	}, nil
}

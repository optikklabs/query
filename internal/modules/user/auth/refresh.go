package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/optikklabs/query/internal/infra/token"
	"github.com/optikklabs/query/internal/modules/user/shared"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

type notUsableError struct {
	reason string
	// familyID is set when a revoked family member was replayed (suspected
	// token theft); the caller revokes the family if nothing else validates.
	familyID string
}

func (e *notUsableError) Error() string { return "refresh token not usable: " + e.reason }

// refreshReuseGrace tolerates replays of a just-rotated token (parallel
// refreshes from the same browser) without treating them as theft.
const refreshReuseGrace = 60 * time.Second

// Refresh validates the cookie candidates and rotates the one that matches.
// The returned string is the new refresh token ("" when rotation was skipped
// inside the reuse grace window).
func (s *Service) Refresh(ctx context.Context, refreshTokens []string, clientIP string) (LoginResponse, string, error) {
	seen := make(map[string]struct{}, len(refreshTokens))
	var reasons []string
	reusedFamilies := make(map[string]struct{})
	for _, raw := range refreshTokens {
		if raw == "" {
			continue
		}
		if _, dup := seen[raw]; dup {
			continue
		}
		seen[raw] = struct{}{}

		response, newRefresh, err := s.refreshOne(ctx, raw)
		if err == nil {
			slog.InfoContext(ctx, "AUTH_EVENT refresh_success",
				slog.Int64("user_id", response.User.ID),
				slog.Int("candidates", len(refreshTokens)),
				slog.String("ip", clientIP))
			return response, newRefresh, nil
		}

		var notUsable *notUsableError
		if !errors.As(err, &notUsable) {
			slog.WarnContext(ctx, "AUTH_EVENT refresh_error",
				slog.Int("candidates", len(refreshTokens)),
				slog.String("ip", clientIP),
				slog.Any("error", err))
			return LoginResponse{}, "", err
		}
		if notUsable.familyID != "" {
			reusedFamilies[notUsable.familyID] = struct{}{}
		}
		reasons = append(reasons, notUsable.reason)
	}

	// Reuse detection: a revoked token was replayed with no valid sibling
	// alongside it; assume theft and revoke every token in its family.
	for familyID := range reusedFamilies {
		if err := s.repo.RevokeFamily(ctx, familyID); err != nil {
			slog.ErrorContext(ctx, "AUTH_EVENT refresh_family_revoke_failed",
				slog.String("family_id", familyID), slog.Any("error", err))
		} else {
			slog.WarnContext(ctx, "AUTH_EVENT refresh_reuse_detected",
				slog.String("family_id", familyID), slog.String("ip", clientIP))
		}
	}

	slog.WarnContext(ctx, "AUTH_EVENT refresh_rejected",
		slog.Int("candidates", len(refreshTokens)),
		slog.Any("reasons", reasons),
		slog.String("ip", clientIP))
	return LoginResponse{}, "", errorcode.UnauthorizedError{Msg: "Invalid or expired refresh token"}
}

func (s *Service) refreshOne(ctx context.Context, refreshToken string) (LoginResponse, string, error) {
	hash := token.HashRefreshToken(refreshToken)
	stored, err := s.repo.FindRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginResponse{}, "", &notUsableError{reason: "unknown_token"}
		}
		return LoginResponse{}, "", fmt.Errorf("failed to look up refresh token: %w", err)
	}

	if time.Now().UTC().After(stored.ExpiresAt) {
		return LoginResponse{}, "", &notUsableError{reason: "expired"}
	}
	// A revoked token is a replay of a rotated credential. Inside the grace
	// window we honor it without rotating; beyond it we flag family reuse.
	rotate := true
	if stored.RevokedAt != nil {
		if time.Since(*stored.RevokedAt) > refreshReuseGrace {
			return LoginResponse{}, "", &notUsableError{reason: "revoked", familyID: stored.FamilyID}
		}
		rotate = false
	}

	user, err := s.repo.FindActiveUserByID(ctx, stored.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginResponse{}, "", &notUsableError{reason: "user_inactive"}
		}
		return LoginResponse{}, "", fmt.Errorf("failed to load user for refresh: %w", err)
	}

	authUser := shared.AuthUser{
		ID:       user.ID,
		Email:    user.Email,
		Name:     user.Name,
		TenantID: user.TenantID,
		Role:     user.Role,
	}

	response, err := s.buildAuthContextResponse(ctx, authUser)
	if err != nil {
		return LoginResponse{}, "", err
	}

	access, err := s.signAccess(authUser, response.Tenant.ID)
	if err != nil {
		return LoginResponse{}, "", err
	}

	var newRefresh string
	if rotate {
		raw, newHash, err := token.GenerateRefreshToken()
		if err != nil {
			return LoginResponse{}, "", fmt.Errorf("failed to issue refresh token: %w", err)
		}
		expiresAt := time.Now().UTC().Add(s.tokens.RefreshTTL())
		if err := s.repo.RotateRefreshToken(ctx, hash, user.ID, stored.FamilyID, newHash, expiresAt); err != nil {
			return LoginResponse{}, "", fmt.Errorf("failed to rotate refresh token: %w", err)
		}
		newRefresh = raw
	}

	return LoginResponse{AuthContextResponse: response, AccessToken: access}, newRefresh, nil
}

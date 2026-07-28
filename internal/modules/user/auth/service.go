package auth

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/optikklabs/query/internal/config"
	"github.com/optikklabs/query/internal/infra/token"
	"github.com/optikklabs/query/internal/modules/user/shared"
	contracts "github.com/optikklabs/query/internal/shared/contracts"
)

type Service struct {
	repo     *Repository
	tokens   *token.Service
	attempts *loginAttempts
	sender   PasswordResetSender
}

type loginAttempts struct {
	mu      sync.Mutex
	entries map[string]attempt
}
type attempt struct {
	failures    int
	lockedUntil time.Time
	lastSeen    time.Time
}

const (
	maxLoginAttemptEntries = 10_000
	loginAttemptTTL        = 15 * time.Minute
)

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
		return LoginResponse{}, "", shared.NewValidationError("Too many login attempts. Try again later.", nil)
	}

	user, err := s.repo.FindActiveUserByEmail(ctx, email)
	if err != nil {
		s.attempts.fail(email, clientIP)
		return LoginResponse{}, "", shared.NewValidationError("Invalid email or password", err)
	}

	if !shared.PasswordIsValid(user.PasswordHash, req.Password) {
		s.attempts.fail(email, clientIP)
		return LoginResponse{}, "", shared.NewValidationError("Invalid email or password", nil)
	}
	s.attempts.reset(email, clientIP)

	response, refresh, err := s.issueTokens(ctx, user, token.NewFamilyID())
	if err != nil {
		return LoginResponse{}, "", err
	}

	slog.InfoContext(ctx, "AUTH_EVENT login_success", slog.Int64("user_id", user.ID), slog.String("email", user.Email), slog.String("ip", clientIP))
	return response, refresh, nil
}

func (l *loginAttempts) key(email, ip string) string { return strings.ToLower(email) + "|" + ip }
func (l *loginAttempts) allow(email, ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	k := l.key(email, ip)
	a, ok := l.entries[k]
	now := time.Now()
	if ok && now.Sub(a.lastSeen) > loginAttemptTTL && !now.Before(a.lockedUntil) {
		delete(l.entries, k)
		return true
	}
	return !now.Before(a.lockedUntil)
}
func (l *loginAttempts) fail(email, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	k := l.key(email, ip)
	if _, exists := l.entries[k]; !exists && len(l.entries) >= maxLoginAttemptEntries {
		l.evictIdleOrOldest(now)
	}
	a := l.entries[k]
	a.failures++
	a.lastSeen = now
	if a.failures >= 5 {
		a.lockedUntil = now.Add(time.Duration(1<<min(a.failures-5, 6)) * time.Minute)
	}
	l.entries[k] = a
}
func (l *loginAttempts) reset(email, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, l.key(email, ip))
}

func (l *loginAttempts) evictIdleOrOldest(now time.Time) {
	var oldestKey string
	var oldest time.Time
	for key, entry := range l.entries {
		if now.Sub(entry.lastSeen) > loginAttemptTTL && !now.Before(entry.lockedUntil) {
			delete(l.entries, key)
			continue
		}
		if oldestKey == "" || entry.lastSeen.Before(oldest) {
			oldestKey = key
			oldest = entry.lastSeen
		}
	}
	if len(l.entries) >= maxLoginAttemptEntries && oldestKey != "" {
		delete(l.entries, oldestKey)
	}
}

type notUsableError struct{ reason string }

func (e *notUsableError) Error() string { return "refresh token not usable: " + e.reason }

func (s *Service) Refresh(ctx context.Context, refreshTokens []string, clientIP string) (LoginResponse, error) {
	seen := make(map[string]struct{}, len(refreshTokens))
	var reasons []string
	for _, raw := range refreshTokens {
		if raw == "" {
			continue
		}
		if _, dup := seen[raw]; dup {
			continue
		}
		seen[raw] = struct{}{}

		response, err := s.refreshOne(ctx, raw)
		if err == nil {
			slog.InfoContext(ctx, "AUTH_EVENT refresh_success",
				slog.Int64("user_id", response.User.ID),
				slog.Int("candidates", len(refreshTokens)),
				slog.String("ip", clientIP))
			return response, nil
		}

		var notUsable *notUsableError
		if !errors.As(err, &notUsable) {
			slog.WarnContext(ctx, "AUTH_EVENT refresh_error",
				slog.Int("candidates", len(refreshTokens)),
				slog.String("ip", clientIP),
				slog.Any("error", err))
			return LoginResponse{}, err
		}
		reasons = append(reasons, notUsable.reason)
	}

	slog.WarnContext(ctx, "AUTH_EVENT refresh_rejected",
		slog.Int("candidates", len(refreshTokens)),
		slog.Any("reasons", reasons),
		slog.String("ip", clientIP))
	return LoginResponse{}, shared.NewUnauthorizedError("Invalid or expired refresh token", nil)
}

func (s *Service) refreshOne(ctx context.Context, refreshToken string) (LoginResponse, error) {
	hash := token.HashRefreshToken(refreshToken)
	stored, err := s.repo.FindRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginResponse{}, &notUsableError{reason: "unknown_token"}
		}
		return LoginResponse{}, shared.NewInternalError("Failed to look up refresh token", err)
	}

	if stored.RevokedAt != nil {
		return LoginResponse{}, &notUsableError{reason: "revoked"}
	}
	if time.Now().UTC().After(stored.ExpiresAt) {
		return LoginResponse{}, &notUsableError{reason: "expired"}
	}

	user, err := s.repo.FindActiveUserByID(ctx, stored.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginResponse{}, &notUsableError{reason: "user_inactive"}
		}
		return LoginResponse{}, shared.NewInternalError("Failed to load user for refresh", err)
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
		return LoginResponse{}, err
	}

	access, err := s.tokens.SignAccess(token.AuthState{
		UserID:          user.ID,
		Email:           user.Email,
		Role:            user.Role,
		DefaultTenantID: response.Tenant.ID,
		TenantIDs:       []int64{response.Tenant.ID},
	})
	if err != nil {
		return LoginResponse{}, shared.NewInternalError("Failed to issue access token", err)
	}

	return LoginResponse{AuthContextResponse: response, AccessToken: access}, nil
}

func (s *Service) IssueTokens(ctx context.Context, user shared.AuthUser) (LoginResponse, string, error) {
	return s.issueTokens(ctx, user, token.NewFamilyID())
}

func (s *Service) issueTokens(ctx context.Context, user shared.AuthUser, familyID string) (LoginResponse, string, error) {
	response, err := s.buildAuthContextResponse(ctx, user)
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
	if err := s.repo.InsertRefreshToken(ctx, user.ID, familyID, hash, expiresAt); err != nil {
		return LoginResponse{}, "", shared.NewInternalError("Failed to issue refresh token", err)
	}

	return LoginResponse{AuthContextResponse: response, AccessToken: access}, raw, nil
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
		return AuthTenantSummary{}, shared.NewValidationError("Account has no associated tenant", nil)
	}

	tenant, err := s.repo.FindTenantByID(ctx, tenantID)
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
		ID:   tenant.ID,
		Name: tenant.Name,

		AccountStatus: tenant.AccountStatus,
		TrialEndsAt:   tenant.TrialEndsAt,
	}, nil
}

func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return shared.NewValidationError("Email is required", nil)
	}

	user, err := s.repo.FindActiveUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {

			return nil
		}
		return shared.NewInternalError("Failed to lookup user", err)
	}

	hash := ""
	if user.PasswordHash != nil {
		hash = *user.PasswordHash
	}

	resetToken, err := s.tokens.SignPasswordReset(user.ID, hash)
	if err != nil {
		return shared.NewInternalError("Failed to generate reset token", err)
	}

	if err := s.sender.SendPasswordReset(ctx, user.Email, resetToken); err != nil {
		return shared.NewInternalError("Failed to send password reset email", err)
	}

	slog.InfoContext(ctx, "AUTH_EVENT forgot_password_requested", slog.Int64("user_id", user.ID))
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, tokenStr string, newPassword string) error {
	if len(newPassword) < shared.MinPasswordLength {
		return shared.NewValidationError("Password must be at least 8 characters", nil)
	}

	userID, err := s.tokens.ExtractSubjectWithoutVerify(tokenStr)
	if err != nil {
		return shared.NewUnauthorizedError("Invalid reset token", err)
	}

	user, err := s.repo.FindAuthUserByID(ctx, userID)
	if err != nil {
		return shared.NewUnauthorizedError("Invalid reset token", err)
	}

	hash := ""
	if user.PasswordHash != nil {
		hash = *user.PasswordHash
	}

	verifiedUserID, err := s.tokens.ParsePasswordReset(tokenStr, hash)
	if err != nil || verifiedUserID != userID {
		return shared.NewUnauthorizedError("Invalid or expired reset token", err)
	}

	newHash, err := shared.HashPassword(newPassword)
	if err != nil {
		return shared.NewInternalError("Failed to hash password", err)
	}

	if err := s.repo.UpdatePasswordAndRevokeSessions(ctx, userID, newHash); err != nil {
		return shared.NewInternalError("Failed to update password and revoke existing sessions", err)
	}

	slog.InfoContext(ctx, "AUTH_EVENT password_reset_success", slog.Int64("user_id", userID))
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error {
	if len(newPassword) < shared.MinPasswordLength {
		return shared.NewValidationError("New password must be at least 8 characters", nil)
	}

	user, err := s.repo.FindAuthUserByID(ctx, userID)
	if err != nil {
		return shared.NewInternalError("Failed to lookup user", err)
	}

	if !shared.PasswordIsValid(user.PasswordHash, currentPassword) {
		return shared.NewValidationError("Invalid current password", nil)
	}

	newHash, err := shared.HashPassword(newPassword)
	if err != nil {
		return shared.NewInternalError("Failed to hash password", err)
	}

	if err := s.repo.UpdatePasswordAndRevokeSessions(ctx, userID, newHash); err != nil {
		return shared.NewInternalError("Failed to update password and revoke existing sessions", err)
	}

	slog.InfoContext(ctx, "AUTH_EVENT password_changed", slog.Int64("user_id", userID))
	return nil
}

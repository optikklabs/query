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
	"golang.org/x/crypto/bcrypt"
)

// Service handles authentication and session issuance.
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

// Login authenticates a user and issues access and refresh tokens.
func (s *Service) Login(ctx context.Context, req LoginRequest, clientIP string) (LoginResponse, string, error) {
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)
	if !s.attempts.allow(email, clientIP) {
		return LoginResponse{}, "", shared.NewValidationError("Too many login attempts. Try again later.", nil)
	}

	user, err := s.repo.FindActiveUserByEmail(ctx, email)
	if err != nil {
		s.attempts.fail(email, clientIP)
		return LoginResponse{}, "", shared.NewValidationError("Invalid email or password", err)
	}

	if user.PasswordHash != nil && *user.PasswordHash != "" && bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)) != nil {
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
	return !time.Now().Before(l.entries[l.key(email, ip)].lockedUntil)
}
func (l *loginAttempts) fail(email, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	k := l.key(email, ip)
	a := l.entries[k]
	a.failures++
	if a.failures >= 5 {
		a.lockedUntil = time.Now().Add(time.Duration(1<<min(a.failures-5, 6)) * time.Minute)
	}
	l.entries[k] = a
}
func (l *loginAttempts) reset(email, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, l.key(email, ip))
}

// errTokenNotUsable marks a candidate refresh token that is simply not valid
// (unknown, revoked, or expired) so Refresh can move on to the next one. It is
// never returned to the caller.
var errTokenNotUsable = errors.New("refresh token not usable")

// Refresh renews a session from any of the presented refresh tokens. A browser
// may hold more than one refresh cookie of the same name (e.g. left over from a
// cookie-path change), so we accept the first that still validates and ignore
// stale siblings. A session is only ever ended by an explicit logout, never by
// a stale token, so refresh can never spuriously log an active user out.
func (s *Service) Refresh(ctx context.Context, refreshTokens []string) (LoginResponse, string, error) {
	seen := make(map[string]struct{}, len(refreshTokens))
	for _, raw := range refreshTokens {
		if raw == "" {
			continue
		}
		if _, dup := seen[raw]; dup {
			continue
		}
		seen[raw] = struct{}{}

		response, refresh, err := s.refreshOne(ctx, raw)
		if err == nil {
			return response, refresh, nil
		}
		// A real internal/decision error (DB down, trial expired) stops here;
		// only an unusable candidate falls through to the next cookie.
		if !errors.Is(err, errTokenNotUsable) {
			return LoginResponse{}, "", err
		}
	}
	return LoginResponse{}, "", shared.NewUnauthorizedError("Invalid or expired refresh token", nil)
}

// refreshOne validates a single refresh token and, if usable, issues a new
// access token and extends the session. It returns errTokenNotUsable when the
// token is unknown, revoked, or expired so the caller can try another.
func (s *Service) refreshOne(ctx context.Context, refreshToken string) (LoginResponse, string, error) {
	hash := token.HashRefreshToken(refreshToken)
	stored, err := s.repo.FindRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginResponse{}, "", errTokenNotUsable
		}
		return LoginResponse{}, "", shared.NewInternalError("Failed to look up refresh token", err)
	}

	if stored.RevokedAt != nil || time.Now().UTC().After(stored.ExpiresAt) {
		return LoginResponse{}, "", errTokenNotUsable
	}

	user, err := s.repo.FindActiveUserByID(ctx, stored.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginResponse{}, "", errTokenNotUsable
		}
		return LoginResponse{}, "", shared.NewInternalError("Failed to load user for refresh", err)
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

	// Extend the session life instead of rotating the refresh token
	expiresAt := time.Now().UTC().Add(s.tokens.RefreshTTL())
	if err := s.repo.ExtendRefreshToken(ctx, hash, expiresAt); err != nil {
		return LoginResponse{}, "", shared.NewInternalError("Failed to extend refresh token", err)
	}

	return LoginResponse{AuthContextResponse: response, AccessToken: access}, refreshToken, nil
}

// IssueTokens mints a fresh session (new token family) for a user. Used by the
// onboarding (signup) and device flows to complete login after they identify the
// user; refresh-token and tenant reads stay owned by auth.
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
	// Revoke every presented token: a browser may hold more than one refresh
	// cookie, and logging out must invalidate the real session, not just the
	// first cookie the browser happened to send.
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
		// Propagate the typed error (e.g. TRIAL_EXPIRED) so callers can react.
		return AuthContextResponse{}, err
	}
	// Role is a property of the user within their tenant, not of the tenant.
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
		// Role is set by the caller from the user's own role.
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
			// Do not leak user existence. Return success.
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
	if len(newPassword) < 8 {
		return shared.NewValidationError("Password must be at least 8 characters", nil)
	}

	// We don't have the user ID or password hash yet, but ParsePasswordReset can't be called without password hash.
	// Wait, to parse the token using a dynamic secret, we need the user's current password hash.
	// But how do we get the user ID to look up the password hash?
	// We can decode the JWT *without* verifying the signature to extract the Subject (user ID).
	// Then look up the user, get their password hash, and *then* fully parse and verify the JWT.
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

	// Fully verify the token now that we have the password hash
	verifiedUserID, err := s.tokens.ParsePasswordReset(tokenStr, hash)
	if err != nil || verifiedUserID != userID {
		return shared.NewUnauthorizedError("Invalid or expired reset token", err)
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return shared.NewInternalError("Failed to hash password", err)
	}

	if err := s.repo.UpdatePassword(ctx, userID, string(newHash)); err != nil {
		return shared.NewInternalError("Failed to update password", err)
	}

	slog.InfoContext(ctx, "AUTH_EVENT password_reset_success", slog.Int64("user_id", userID))
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return shared.NewValidationError("New password must be at least 8 characters", nil)
	}

	user, err := s.repo.FindAuthUserByID(ctx, userID)
	if err != nil {
		return shared.NewInternalError("Failed to lookup user", err)
	}

	if user.PasswordHash == nil || *user.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(currentPassword)) != nil {
		return shared.NewValidationError("Invalid current password", nil)
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return shared.NewInternalError("Failed to hash password", err)
	}

	if err := s.repo.UpdatePassword(ctx, userID, string(newHash)); err != nil {
		return shared.NewInternalError("Failed to update password", err)
	}

	slog.InfoContext(ctx, "AUTH_EVENT password_changed", slog.Int64("user_id", userID))
	return nil
}

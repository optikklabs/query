package signup

import (
	"context"
	"log/slog"
	"strings"

	"github.com/optikklabs/query/internal/modules/user/auth"
	"github.com/optikklabs/query/internal/modules/user/shared"
	"golang.org/x/crypto/bcrypt"
)

// minPasswordLength mirrors the web client's rule; the server is the source of
// truth so API callers (CLI) can't bypass it.
const minPasswordLength = 8

// Service provisions a new account + tenant, then delegates session issuance to
// auth. It composes tenant creation, user creation, and token minting.
type Service struct {
	repo   *Repository
	issuer *auth.Service
}

func NewService(repo *Repository, issuer *auth.Service) *Service {
	return &Service{repo: repo, issuer: issuer}
}

// Signup creates the tenant and its first admin user atomically, then issues a
// session. Returns the response (including api_key) and the raw refresh token.
func (s *Service) Signup(ctx context.Context, req SignupRequest) (SignupResponse, string, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))
	name := strings.TrimSpace(req.Name)
	tenantName := strings.TrimSpace(req.TenantName)
	password := strings.TrimSpace(req.Password)

	if err := validateSignup(email, name, tenantName, password); err != nil {
		return SignupResponse{}, "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return SignupResponse{}, "", shared.NewInternalError("Failed to hash password", err)
	}

	apiKey, err := shared.GenerateAPIKey()
	if err != nil {
		return SignupResponse{}, "", shared.NewInternalError("Failed to generate api key", err)
	}

	tenantID, userID, err := s.repo.CreateTenantWithAdmin(ctx, tenantName, apiKey, email, string(hash), name)
	if err != nil {
		if IsDuplicateEmail(err) {
			return SignupResponse{}, "", shared.NewConflictError("An account with this email already exists", err)
		}
		return SignupResponse{}, "", shared.NewInternalError("Failed to create account", err)
	}

	session, refresh, err := s.issuer.IssueTokens(shared.AuthUser{
		ID:       userID,
		Email:    email,
		Name:     name,
		TenantID: tenantID,
	})
	if err != nil {
		return SignupResponse{}, "", err
	}

	slog.InfoContext(ctx, "AUTH_EVENT signup_success",
		slog.Int64("user_id", userID), slog.Int64("tenant_id", tenantID), slog.String("email", email))
	return SignupResponse{LoginResponse: session, APIKey: apiKey}, refresh, nil
}

func validateSignup(email, name, tenantName, password string) error {
	switch {
	case email == "" || !strings.Contains(email, "@"):
		return shared.NewValidationError("A valid email is required", nil)
	case name == "":
		return shared.NewValidationError("Your name is required", nil)
	case tenantName == "":
		return shared.NewValidationError("An organization name is required", nil)
	case len(password) < minPasswordLength:
		return shared.NewValidationError("Password must be at least 8 characters", nil)
	}
	return nil
}

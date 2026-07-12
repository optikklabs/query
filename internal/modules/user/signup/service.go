package signup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/modules/user/auth"
	"github.com/optikklabs/query/internal/modules/user/shared"
	"golang.org/x/crypto/bcrypt"
)

// minPasswordLength mirrors the web client's rule; the server is the source of
// truth so API callers (CLI) can't bypass it.
const minPasswordLength = 8
const verificationTTL = 24 * time.Hour

// trialDuration is the free-trial window a new tenant starts with.
const trialDuration = 7 * 24 * time.Hour

// Service provisions a new account + tenant, then delegates session issuance to
// auth. It composes tenant creation, user creation, and token minting.
type Service struct {
	repo          *Repository
	issuer        *auth.Service
	resendAPIKey  string
	mailFrom      string
	verifyBaseURL string
	httpClient    *http.Client
}

func NewService(repo *Repository, issuer *auth.Service, resendAPIKey, mailFrom, verifyBaseURL string) *Service {
	return &Service{repo: repo, issuer: issuer, resendAPIKey: resendAPIKey, mailFrom: mailFrom, verifyBaseURL: verifyBaseURL, httpClient: &http.Client{Timeout: 5 * time.Second}}
}

// Signup creates the tenant and its first admin user atomically, then issues a
// session. Returns the response (including api_key) and the raw refresh token.
func (s *Service) Signup(ctx context.Context, req SignupRequest) (SignupResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))
	name := strings.TrimSpace(req.Name)
	tenantName := strings.TrimSpace(req.TenantName)
	password := strings.TrimSpace(req.Password)

	if err := validateSignup(email, name, tenantName, password); err != nil {
		return SignupResponse{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return SignupResponse{}, shared.NewInternalError("Failed to hash password", err)
	}

	apiKey, err := shared.GenerateAPIKey()
	if err != nil {
		return SignupResponse{}, shared.NewInternalError("Failed to generate api key", err)
	}
	verificationToken, err := shared.GenerateDeviceCode()
	if err != nil {
		return SignupResponse{}, shared.NewInternalError("Failed to generate verification token", err)
	}
	sum := sha256.Sum256([]byte(verificationToken))

	trialEndsAt := time.Now().UTC().Add(trialDuration)
	tenantID, userID, err := s.repo.CreateTenantWithAdmin(ctx, tenantName, apiKey, email, string(hash), name, hex.EncodeToString(sum[:]), time.Now().UTC().Add(verificationTTL), trialEndsAt)
	if err != nil {
		if IsDuplicateEmail(err) {
			return SignupResponse{}, shared.NewConflictError("An account with this email already exists", err)
		}
		return SignupResponse{}, shared.NewInternalError("Failed to create account", err)
	}
	if err := s.sendVerification(ctx, email, verificationToken); err != nil {
		return SignupResponse{}, shared.NewInternalError("Failed to send verification email", err)
	}

	slog.InfoContext(ctx, "AUTH_EVENT signup_success",
		slog.Int64("user_id", userID), slog.Int64("tenant_id", tenantID), slog.String("email", email))
	return SignupResponse{Message: "Check your email to verify your account."}, nil
}

func (s *Service) VerifyEmail(ctx context.Context, rawToken string) (auth.LoginResponse, string, string, error) {
	sum := sha256.Sum256([]byte(strings.TrimSpace(rawToken)))
	user, err := s.repo.ConsumeVerification(ctx, hex.EncodeToString(sum[:]))
	if err != nil {
		return auth.LoginResponse{}, "", "", shared.NewValidationError("Verification link is invalid or expired", err)
	}
	apiKey, err := shared.GenerateAPIKey()
	if err != nil {
		return auth.LoginResponse{}, "", "", shared.NewInternalError("Failed to create API key", err)
	}
	if err := s.repo.RotateTenantAPIKey(ctx, user.TenantID, apiKey); err != nil {
		return auth.LoginResponse{}, "", "", shared.NewInternalError("Failed to activate account", err)
	}
	session, refresh, err := s.issuer.IssueTokens(user)
	if err != nil {
		return auth.LoginResponse{}, "", "", err
	}
	return session, refresh, apiKey, nil
}

func (s *Service) sendVerification(ctx context.Context, to, token string) error {
	verifyURL := s.verifyBaseURL + "?token=" + url.QueryEscape(token)
	body, _ := json.Marshal(map[string]any{"from": s.mailFrom, "to": []string{to}, "subject": "Verify your Optikk email", "html": "<p>Verify your account by opening <a href=\"" + verifyURL + "\">this link</a>. This link expires in 24 hours.</p>"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.resendAPIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("email provider returned %s", resp.Status)
	}
	return nil
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

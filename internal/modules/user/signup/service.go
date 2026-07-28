package signup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/config"
	emailinfra "github.com/optikklabs/query/internal/infra/email"
	"github.com/optikklabs/query/internal/modules/user/auth"
	"github.com/optikklabs/query/internal/modules/user/shared"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

const verificationTTL = 24 * time.Hour

const trialDuration = 7 * 24 * time.Hour

const termsVersion = "2026-07-14"

type Service struct {
	repo                 *Repository
	issuer               *auth.Service
	verificationRequired bool
	sender               VerificationSender
}

type VerificationSender interface {
	SendVerification(ctx context.Context, to, token string) error
}

type ResendVerificationSender struct {
	verifyBaseURL string
	mailer        *emailinfra.ResendSender
}

func NewService(repo *Repository, issuer *auth.Service, email config.EmailConfig) *Service {
	sender := VerificationSender(noopVerificationSender{})
	if email.ResendVerificationEnabled {
		sender = NewResendVerificationSender(email.ResendAPIKey, email.From, email.VerifyBaseURL)
	}
	return &Service{
		repo:                 repo,
		issuer:               issuer,
		verificationRequired: email.ResendVerificationEnabled,
		sender:               sender,
	}
}

func NewResendVerificationSender(apiKey, from, verifyBaseURL string) *ResendVerificationSender {
	return &ResendVerificationSender{
		verifyBaseURL: verifyBaseURL,
		mailer:        emailinfra.NewResendSender(apiKey, from),
	}
}

type noopVerificationSender struct{}

func (noopVerificationSender) SendVerification(context.Context, string, string) error {
	return nil
}

type SignupResult struct {
	Message      string
	Session      *auth.LoginResponse
	RefreshToken string
	APIKey       string
}

type normalizedSignup struct {
	email         string
	name          string
	tenantName    string
	password      string
	acceptedTerms bool
}

type signupSecrets struct {
	passwordHash       string
	apiKey             string
	verificationToken  string
	verificationHash   string
	verificationExpiry time.Time
}

func (s *Service) Signup(ctx context.Context, req SignupRequest) (SignupResult, error) {
	normalized, err := normalizeSignup(req)
	if err != nil {
		return SignupResult{}, err
	}

	secrets, err := s.prepareSignupSecrets(normalized.password)
	if err != nil {
		return SignupResult{}, err
	}

	active := !s.verificationRequired
	trialEndsAt := time.Now().UTC().Add(trialDuration)
	acceptedAt := time.Now().UTC()
	user, err := s.provisionSignup(ctx, normalized, secrets, active, trialEndsAt, acceptedAt)
	if err != nil {
		return SignupResult{}, err
	}

	if s.verificationRequired {
		if err := s.sender.SendVerification(ctx, normalized.email, secrets.verificationToken); err != nil {
			return SignupResult{}, fmt.Errorf("Failed to send verification email: %w", err)
		}
		slog.InfoContext(ctx, "AUTH_EVENT signup_success",
			slog.Int64("user_id", user.ID), slog.Int64("tenant_id", user.TenantID), slog.String("email", user.Email))
		return SignupResult{Message: "Check your email to verify your account."}, nil
	}

	session, refresh, err := s.issuer.IssueTokens(ctx, user)
	if err != nil {
		return SignupResult{}, err
	}
	slog.InfoContext(ctx, "AUTH_EVENT signup_success",
		slog.Int64("user_id", user.ID), slog.Int64("tenant_id", user.TenantID), slog.String("email", user.Email))
	return SignupResult{Session: &session, RefreshToken: refresh, APIKey: secrets.apiKey}, nil
}

func (s *Service) VerifyEmail(ctx context.Context, rawToken string) (auth.LoginResponse, string, string, error) {
	sum := sha256.Sum256([]byte(strings.TrimSpace(rawToken)))
	user, err := s.repo.ConsumeVerification(ctx, hex.EncodeToString(sum[:]))
	if err != nil {
		return auth.LoginResponse{}, "", "", errorcode.ValidationError{Msg: "Verification link is invalid or expired"}
	}
	apiKey, err := shared.GenerateAPIKey()
	if err != nil {
		return auth.LoginResponse{}, "", "", fmt.Errorf("Failed to create API key: %w", err)
	}
	if err := s.repo.RotateTenantAPIKey(ctx, user.TenantID, apiKey); err != nil {
		return auth.LoginResponse{}, "", "", fmt.Errorf("Failed to activate account: %w", err)
	}
	session, refresh, err := s.issuer.IssueTokens(ctx, user)
	if err != nil {
		return auth.LoginResponse{}, "", "", err
	}
	return session, refresh, apiKey, nil
}

func (s *Service) prepareSignupSecrets(password string) (signupSecrets, error) {
	hash, err := shared.HashPassword(password)
	if err != nil {
		return signupSecrets{}, fmt.Errorf("Failed to hash password: %w", err)
	}
	secrets := signupSecrets{passwordHash: hash}
	if !s.verificationRequired {
		apiKey, err := shared.GenerateAPIKey()
		if err != nil {
			return signupSecrets{}, fmt.Errorf("Failed to generate api key: %w", err)
		}
		secrets.apiKey = apiKey
		return secrets, nil
	}
	// Verified signups get their real key at verify time (VerifyEmail
	// rotates it); until then store an unusable revoked sentinel.
	sentinel, err := shared.GenerateRevokedKey()
	if err != nil {
		return signupSecrets{}, fmt.Errorf("Failed to generate api key: %w", err)
	}
	secrets.apiKey = sentinel
	token, err := shared.GenerateDeviceCode()
	if err != nil {
		return signupSecrets{}, fmt.Errorf("Failed to generate verification token: %w", err)
	}
	sum := sha256.Sum256([]byte(token))
	secrets.verificationToken = token
	secrets.verificationHash = hex.EncodeToString(sum[:])
	secrets.verificationExpiry = time.Now().UTC().Add(verificationTTL)
	return secrets, nil
}

func (s *Service) provisionSignup(ctx context.Context, req normalizedSignup, secrets signupSecrets, active bool, trialEndsAt, acceptedAt time.Time) (shared.AuthUser, error) {
	signupRow := tenantAdminSignup{
		TenantName:         req.tenantName,
		APIKey:             secrets.apiKey,
		Email:              req.email,
		PasswordHash:       secrets.passwordHash,
		UserName:           req.name,
		Active:             active,
		VerificationHash:   secrets.verificationHash,
		VerificationExpiry: secrets.verificationExpiry,
		TrialEndsAt:        trialEndsAt,
		TermsAcceptedAt:    acceptedAt,
		TermsVersion:       termsVersion,
	}
	user, err := s.repo.CreateTenantWithAdmin(ctx, signupRow)
	if err == nil {
		return user, nil
	}
	if !IsDuplicateEmail(err) {
		return shared.AuthUser{}, fmt.Errorf("Failed to create account: %w", err)
	}

	user, updateErr := s.repo.UpdateUnverifiedTenantAndAdmin(ctx, signupRow)
	if updateErr != nil {
		if errors.Is(updateErr, ErrAlreadyVerified) {
			return shared.AuthUser{}, errorcode.ConflictError{Msg: "An account with this email already exists"}
		}
		return shared.AuthUser{}, fmt.Errorf("Failed to update unverified account: %w", updateErr)
	}
	return user, nil
}

func (s *ResendVerificationSender) SendVerification(ctx context.Context, to, token string) error {
	verifyURL := s.verifyBaseURL + "?token=" + url.QueryEscape(token)
	html := "<p>Verify your account by opening <a href=\"" + verifyURL + "\">this link</a>. This link expires in 24 hours.</p>"
	return s.mailer.Send(ctx, to, "Verify your Optikk email", html)
}

func normalizeSignup(req SignupRequest) (normalizedSignup, error) {
	normalized := normalizedSignup{
		email:         strings.TrimSpace(strings.ToLower(req.Email)),
		name:          strings.TrimSpace(req.Name),
		tenantName:    strings.TrimSpace(req.TenantName),
		password:      req.Password,
		acceptedTerms: req.AcceptedTerms,
	}
	if err := validateSignup(normalized); err != nil {
		return normalizedSignup{}, err
	}
	return normalized, nil
}

func validateSignup(s normalizedSignup) error {
	switch {
	case s.email == "" || !strings.Contains(s.email, "@"):
		return errorcode.ValidationError{Msg: "A valid email is required"}
	case s.name == "":
		return errorcode.ValidationError{Msg: "Your name is required"}
	case s.tenantName == "":
		return errorcode.ValidationError{Msg: "An organization name is required"}
	case len(s.password) < shared.MinPasswordLength:
		return errorcode.ValidationError{Msg: "Password must be at least 8 characters"}
	case !s.acceptedTerms:
		return errorcode.ValidationError{Msg: "You must accept the Terms of Service and Privacy Policy"}
	}
	return nil
}

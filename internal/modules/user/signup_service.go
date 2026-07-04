package user

import (
	"context"
	"log/slog"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/optikklabs/query/internal/infra/token"
	"golang.org/x/crypto/bcrypt"
)

// NIST SP 800-63B minimum memorized-secret length.
const minPasswordLength = 8

// Signup self-serves a new customer: org team + admin user + session in one call.
func (s *Service) Signup(ctx context.Context, req SignupRequest, clientIP string) (SignupResponse, string, error) {
	req.Email = strings.TrimSpace(req.Email)
	req.Name = strings.TrimSpace(req.Name)
	req.OrgName = strings.TrimSpace(req.OrgName)

	if err := validateSignup(req); err != nil {
		return SignupResponse{}, "", err
	}

	if _, err := s.repo.FindActiveUserByEmail(req.Email); err == nil {
		return SignupResponse{}, "", NewConflictError("An account with this email already exists", nil)
	}

	apiKey, err := GenerateAPIKey()
	if err != nil {
		return SignupResponse{}, "", NewInternalError("Failed to generate api key", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return SignupResponse{}, "", NewInternalError("Failed to hash password", err)
	}

	// One transaction: a failed user insert rolls back the team (no orphans).
	if _, _, err := s.repo.SignupTeamAndAdmin(ctx, NewSignupTeam{
		OrgName:      req.OrgName,
		Name:         req.OrgName,
		Slug:         deriveSlug(req.OrgName),
		Color:        "#3B82F6",
		APIKey:       apiKey,
		Email:        req.Email,
		UserName:     req.Name,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	}); err != nil {
		return SignupResponse{}, "", mapSignupInsertError(err)
	}

	user, err := s.repo.FindActiveUserByEmail(req.Email)
	if err != nil {
		return SignupResponse{}, "", NewInternalError("Failed to load created account", err)
	}

	response, refresh, err := s.issueTokens(user, token.NewFamilyID())
	if err != nil {
		return SignupResponse{}, "", err
	}

	slog.InfoContext(ctx, "AUTH_EVENT signup_success", slog.Int64("user_id", user.ID), slog.String("email", user.Email), slog.String("ip", clientIP))
	return SignupResponse{LoginResponse: response, APIKey: apiKey}, refresh, nil
}

// mapSignupInsertError translates a duplicate-key insert error into a
// user-facing conflict, distinguishing the org-name and email unique keys.
func mapSignupInsertError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "1062") || strings.Contains(msg, "Duplicate entry") {
		if strings.Contains(msg, "uq_team_org_name") {
			return NewConflictError("Organization name is already taken", err)
		}
		return NewConflictError("An account with this email already exists", err)
	}
	return NewInternalError("Failed to create account", err)
}

func validateSignup(req SignupRequest) error {
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return NewValidationError("A valid email is required", err)
	}
	if len(req.Password) < minPasswordLength {
		return NewValidationError("Password must be at least 8 characters", nil)
	}
	if req.Name == "" || req.OrgName == "" {
		return NewValidationError("name and org_name are required", nil)
	}
	return nil
}

// signupLimiter is a fixed-window per-IP limiter for the public signup route.
type signupLimiter struct {
	mu     sync.Mutex
	byIP   map[string]*signupWindow
	limit  int
	window time.Duration
}

type signupWindow struct {
	count   int
	startAt time.Time
}

func newSignupLimiter(limit int, window time.Duration) *signupLimiter {
	return &signupLimiter{byIP: make(map[string]*signupWindow), limit: limit, window: window}
}

func (l *signupLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	w, ok := l.byIP[ip]
	if !ok || now.Sub(w.startAt) > l.window {
		// Piggyback expired-entry cleanup on window rollover.
		for k, v := range l.byIP {
			if now.Sub(v.startAt) > l.window {
				delete(l.byIP, k)
			}
		}
		l.byIP[ip] = &signupWindow{count: 1, startAt: now}
		return true
	}
	if w.count >= l.limit {
		return false
	}
	w.count++
	return true
}

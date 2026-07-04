package user

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/optikklabs/query/internal/infra/token"
)

// RFC 8628 device-flow tuning: codes live 10m, clients poll every 5s.
const (
	deviceCodeTTL   = 10 * time.Minute
	devicePollEvery = 5 * time.Second
)

// RFC 8628 §3.5 poll error codes, surfaced verbatim to the CLI.
var (
	ErrDeviceAuthPending = errors.New("authorization_pending")
	ErrDeviceSlowDown    = errors.New("slow_down")
	ErrDeviceExpired     = errors.New("expired_token")
)

// StartDeviceAuth mints a device/user code pair for the browser handoff.
func (s *Service) StartDeviceAuth(ctx context.Context) (DeviceCodeResponse, error) {
	deviceCode, err := GenerateDeviceCode()
	if err != nil {
		return DeviceCodeResponse{}, NewInternalError("Failed to generate device code", err)
	}
	userCode, err := GenerateUserCode()
	if err != nil {
		return DeviceCodeResponse{}, NewInternalError("Failed to generate user code", err)
	}
	expiresAt := time.Now().UTC().Add(deviceCodeTTL)
	if err := s.repo.InsertDeviceCode(ctx, deviceCode, userCode, expiresAt); err != nil {
		return DeviceCodeResponse{}, NewInternalError("Failed to store device code", err)
	}
	return DeviceCodeResponse{
		DeviceCode: deviceCode,
		UserCode:   userCode,
		Interval:   int(devicePollEvery.Seconds()),
		ExpiresIn:  int(deviceCodeTTL.Seconds()),
	}, nil
}

// PollDeviceToken resolves a device_code to a session once the user approves,
// returning RFC 8628 sentinel errors while pending/expired/too-fast.
func (s *Service) PollDeviceToken(ctx context.Context, deviceCode string) (LoginResponse, string, error) {
	record, err := s.repo.FindDeviceCode(ctx, deviceCode)
	if err != nil {
		return LoginResponse{}, "", ErrDeviceExpired
	}

	now := time.Now().UTC()
	if status := evaluateDeviceCode(record, now); status != nil {
		// Record the poll only when we didn't reject it as too-fast.
		if !errors.Is(status, ErrDeviceSlowDown) {
			if err := s.repo.TouchDeviceCodePolled(ctx, deviceCode, now); err != nil {
				slog.WarnContext(ctx, "AUTH_EVENT device_poll_touch_failed", slog.Any("error", err))
			}
		}
		return LoginResponse{}, "", status
	}
	if err := s.repo.TouchDeviceCodePolled(ctx, deviceCode, now); err != nil {
		slog.WarnContext(ctx, "AUTH_EVENT device_poll_touch_failed", slog.Any("error", err))
	}

	user, err := s.repo.FindActiveUserByID(*record.UserID)
	if err != nil {
		return LoginResponse{}, "", NewUnauthorizedError("Approved user is no longer active", err)
	}
	if err := s.repo.ConsumeDeviceCode(ctx, deviceCode, now); err != nil {
		return LoginResponse{}, "", NewInternalError("Failed to consume device code", err)
	}

	authUser := AuthUser{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		AvatarURL: user.AvatarURL,
		TeamsJSON: user.TeamsJSON,
	}
	response, refresh, err := s.issueTokens(authUser, token.NewFamilyID())
	if err != nil {
		return LoginResponse{}, "", err
	}
	slog.InfoContext(ctx, "AUTH_EVENT device_login_success", slog.Int64("user_id", user.ID), slog.String("email", user.Email))
	return response, refresh, nil
}

// evaluateDeviceCode is the pure poll state machine: nil means the code is
// approved and ready to exchange; otherwise it returns the RFC 8628 sentinel.
func evaluateDeviceCode(record DeviceCodeRecord, now time.Time) error {
	switch {
	case now.After(record.ExpiresAt) || record.ConsumedAt != nil:
		return ErrDeviceExpired
	case record.LastPolledAt != nil && now.Sub(*record.LastPolledAt) < devicePollEvery:
		return ErrDeviceSlowDown
	case record.ApprovedAt == nil || record.UserID == nil:
		return ErrDeviceAuthPending
	default:
		return nil
	}
}

// ApproveDeviceCode binds a pending user_code to the authenticated user.
func (s *Service) ApproveDeviceCode(ctx context.Context, userCode string, userID int64) error {
	record, err := s.repo.FindDeviceCodeByUserCode(ctx, userCode)
	if err != nil {
		return NewValidationError("Unknown or expired code", err)
	}
	if time.Now().UTC().After(record.ExpiresAt) || record.ConsumedAt != nil {
		return NewValidationError("This code has expired. Start a new login from the CLI.", nil)
	}
	if record.ApprovedAt != nil {
		return NewValidationError("This code was already approved.", nil)
	}
	if err := s.repo.ApproveDeviceCode(ctx, userCode, userID, time.Now().UTC()); err != nil {
		return NewInternalError("Failed to approve device code", err)
	}
	return nil
}

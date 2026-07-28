package device

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/optikklabs/query/internal/modules/user/auth"
	"github.com/optikklabs/query/internal/modules/user/shared"
)

const (
	deviceCodeTTL   = 10 * time.Minute
	devicePollEvery = 5 * time.Second
)

var (
	ErrDeviceAuthPending = errors.New("authorization_pending")
	ErrDeviceSlowDown    = errors.New("slow_down")
	ErrDeviceExpired     = errors.New("expired_token")
)

type Service struct {
	repo   *Repository
	issuer *auth.Service
}

func NewService(repo *Repository, issuer *auth.Service) *Service {
	return &Service{repo: repo, issuer: issuer}
}

func (s *Service) StartDeviceAuth(ctx context.Context) (DeviceCodeResponse, error) {
	deviceCode, err := shared.GenerateDeviceCode()
	if err != nil {
		return DeviceCodeResponse{}, shared.NewInternalError("Failed to generate device code", err)
	}
	userCode, err := shared.GenerateUserCode()
	if err != nil {
		return DeviceCodeResponse{}, shared.NewInternalError("Failed to generate user code", err)
	}
	expiresAt := time.Now().UTC().Add(deviceCodeTTL)
	if err := s.repo.InsertDeviceCode(ctx, deviceCode, userCode, expiresAt); err != nil {
		return DeviceCodeResponse{}, shared.NewInternalError("Failed to store device code", err)
	}
	return DeviceCodeResponse{
		DeviceCode: deviceCode,
		UserCode:   userCode,
		Interval:   int(devicePollEvery.Seconds()),
		ExpiresIn:  int(deviceCodeTTL.Seconds()),
	}, nil
}

func (s *Service) PollDeviceToken(ctx context.Context, deviceCode string) (auth.LoginResponse, string, error) {
	record, err := s.repo.FindDeviceCode(ctx, deviceCode)
	if err != nil {
		return auth.LoginResponse{}, "", ErrDeviceExpired
	}

	now := time.Now().UTC()
	if status := evaluateDeviceCode(record, now); status != nil {

		if !errors.Is(status, ErrDeviceSlowDown) {
			if err := s.repo.TouchDeviceCodePolled(ctx, deviceCode, now); err != nil {
				slog.WarnContext(ctx, "AUTH_EVENT device_poll_touch_failed", slog.Any("error", err))
			}
		}
		return auth.LoginResponse{}, "", status
	}
	if err := s.repo.TouchDeviceCodePolled(ctx, deviceCode, now); err != nil {
		slog.WarnContext(ctx, "AUTH_EVENT device_poll_touch_failed", slog.Any("error", err))
	}

	user, err := s.repo.FindActiveUserByID(ctx, *record.UserID)
	if err != nil {
		return auth.LoginResponse{}, "", shared.NewUnauthorizedError("Approved user is no longer active", err)
	}
	if err := s.repo.ConsumeDeviceCode(ctx, deviceCode, now); err != nil {
		return auth.LoginResponse{}, "", shared.NewInternalError("Failed to consume device code", err)
	}

	authUser := shared.AuthUser{
		ID:       user.ID,
		Email:    user.Email,
		Name:     user.Name,
		TenantID: user.TenantID,
	}
	response, refresh, err := s.issuer.IssueTokens(ctx, authUser)
	if err != nil {
		return auth.LoginResponse{}, "", err
	}
	slog.InfoContext(ctx, "AUTH_EVENT device_login_success", slog.Int64("user_id", user.ID), slog.String("email", user.Email))
	return response, refresh, nil
}

func evaluateDeviceCode(record shared.DeviceCodeRecord, now time.Time) error {
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

func (s *Service) ApproveDeviceCode(ctx context.Context, userCode string, userID int64) error {
	record, err := s.repo.FindDeviceCodeByUserCode(ctx, userCode)
	if err != nil {
		return shared.NewValidationError("Unknown or expired code", err)
	}
	if time.Now().UTC().After(record.ExpiresAt) || record.ConsumedAt != nil {
		return shared.NewValidationError("This code has expired. Start a new login from the CLI.", nil)
	}
	if record.ApprovedAt != nil {
		return shared.NewValidationError("This code was already approved.", nil)
	}
	if err := s.repo.ApproveDeviceCode(ctx, userCode, userID, time.Now().UTC()); err != nil {
		return shared.NewInternalError("Failed to approve device code", err)
	}
	return nil
}

package device

import (
	"errors"
	"testing"
	"time"

	"github.com/optikklabs/query/internal/modules/user/shared"
)

func TestEvaluateDeviceCode(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	userID := int64(7)
	approved := now.Add(-time.Second)
	future := now.Add(deviceCodeTTL)
	past := now.Add(-time.Minute)

	tests := []struct {
		name    string
		record  shared.DeviceCodeRecord
		wantErr error
	}{
		{
			name:    "expired",
			record:  shared.DeviceCodeRecord{ExpiresAt: past},
			wantErr: ErrDeviceExpired,
		},
		{
			name:    "already consumed",
			record:  shared.DeviceCodeRecord{ExpiresAt: future, ConsumedAt: &now},
			wantErr: ErrDeviceExpired,
		},
		{
			name:    "polled too fast",
			record:  shared.DeviceCodeRecord{ExpiresAt: future, LastPolledAt: &now},
			wantErr: ErrDeviceSlowDown,
		},
		{
			name:    "pending approval",
			record:  shared.DeviceCodeRecord{ExpiresAt: future, LastPolledAt: &past},
			wantErr: ErrDeviceAuthPending,
		},
		{
			name:    "approved and ready",
			record:  shared.DeviceCodeRecord{ExpiresAt: future, LastPolledAt: &past, ApprovedAt: &approved, UserID: &userID},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := evaluateDeviceCode(tt.record, now); !errors.Is(err, tt.wantErr) {
				t.Errorf("evaluateDeviceCode() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

package user

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// A user code must be unambiguous, dash-formatted, and unique per draw.
func TestGenerateUserCode(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code, err := GenerateUserCode()
		if err != nil {
			t.Fatalf("GenerateUserCode: %v", err)
		}
		if len(code) != 9 || code[4] != '-' {
			t.Fatalf("code %q not in XXXX-XXXX form", code)
		}
		for _, c := range code {
			if c != '-' && !strings.ContainsRune(userCodeAlphabet, c) {
				t.Fatalf("code %q has out-of-alphabet char %q", code, c)
			}
		}
		if seen[code] {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = true
	}
}

func TestGenerateDeviceCode(t *testing.T) {
	code, err := GenerateDeviceCode()
	if err != nil {
		t.Fatalf("GenerateDeviceCode: %v", err)
	}
	if len(code) != 64 {
		t.Errorf("device code length = %d, want 64", len(code))
	}
}

func TestEvaluateDeviceCode(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	userID := int64(7)
	approved := now.Add(-time.Second)
	future := now.Add(deviceCodeTTL)
	past := now.Add(-time.Minute)

	tests := []struct {
		name    string
		record  DeviceCodeRecord
		wantErr error
	}{
		{
			name:    "expired",
			record:  DeviceCodeRecord{ExpiresAt: past},
			wantErr: ErrDeviceExpired,
		},
		{
			name:    "already consumed",
			record:  DeviceCodeRecord{ExpiresAt: future, ConsumedAt: &now},
			wantErr: ErrDeviceExpired,
		},
		{
			name:    "polled too fast",
			record:  DeviceCodeRecord{ExpiresAt: future, LastPolledAt: &now},
			wantErr: ErrDeviceSlowDown,
		},
		{
			name:    "pending approval",
			record:  DeviceCodeRecord{ExpiresAt: future, LastPolledAt: &past},
			wantErr: ErrDeviceAuthPending,
		},
		{
			name:    "approved and ready",
			record:  DeviceCodeRecord{ExpiresAt: future, LastPolledAt: &past, ApprovedAt: &approved, UserID: &userID},
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

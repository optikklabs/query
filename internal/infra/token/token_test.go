package token

import (
	"bytes"
	"testing"
)

func TestPasswordResetRoundTrip(t *testing.T) {
	// Spare capacity on the shared secret is what made in-place appends unsafe.
	secret := make([]byte, 32, 256)
	copy(secret, "0123456789abcdef0123456789abcdef")
	svc := &Service{secret: secret}
	original := bytes.Clone(secret[:cap(secret)])

	const hash = "$2a$10$abcdefghijklmnopqrstuuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0"
	raw, err := svc.SignPasswordReset(42, hash)
	if err != nil {
		t.Fatalf("SignPasswordReset: %v", err)
	}

	if !bytes.Equal(secret[:cap(secret)], original) {
		t.Fatal("signing wrote into the shared secret's backing array")
	}

	userID, err := svc.ParsePasswordReset(raw, hash)
	if err != nil {
		t.Fatalf("ParsePasswordReset: %v", err)
	}
	if userID != 42 {
		t.Fatalf("userID = %d, want 42", userID)
	}

	if _, err := svc.ParsePasswordReset(raw, hash+"changed"); err == nil {
		t.Fatal("token still verified after the password hash changed")
	}
}

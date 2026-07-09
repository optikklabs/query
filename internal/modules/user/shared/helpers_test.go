package shared

import (
	"strings"
	"testing"
)

// Keys must carry the scan-friendly prefix and 256 bits of hex-encoded entropy.
func TestGenerateAPIKeyFormat(t *testing.T) {
	key, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if !strings.HasPrefix(key, "ok_") {
		t.Errorf("key %q missing ok_ prefix", key)
	}
	if got, want := len(key), len("ok_")+64; got != want {
		t.Errorf("key length = %d, want %d", got, want)
	}
	for _, c := range key[len("ok_"):] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("key %q has non-hex char %q", key, c)
		}
	}
}

// Two consecutive keys colliding would mean the RNG is broken.
func TestGenerateAPIKeyUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey: %v", err)
		}
		if seen[key] {
			t.Fatalf("duplicate key %q", key)
		}
		seen[key] = true
	}
}

// A revoke sentinel must be unique, prefixed, and never mistaken for a live key.
func TestGenerateRevokedKey(t *testing.T) {
	key, err := GenerateRevokedKey()
	if err != nil {
		t.Fatalf("GenerateRevokedKey: %v", err)
	}
	if !strings.HasPrefix(key, revokedKeyPrefix) {
		t.Errorf("revoked key %q not recognized as revoked", key)
	}
	if got, want := len(key), len(revokedKeyPrefix)+64; got != want {
		t.Errorf("revoked key length = %d, want %d", got, want)
	}
}


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

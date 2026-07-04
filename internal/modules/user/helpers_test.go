package user

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
	if !IsRevokedAPIKey(key) {
		t.Errorf("revoked key %q not recognized as revoked", key)
	}
	if got, want := len(key), len(revokedKeyPrefix)+64; got != want {
		t.Errorf("revoked key length = %d, want %d", got, want)
	}
}

func TestIsRevokedAPIKey(t *testing.T) {
	cases := []struct {
		name, in string
		want     bool
	}{
		{"live ok_ key", "ok_deadbeef", false},
		{"legacy hex key", "a1b2c3d4", false},
		{"revoked sentinel", "revoked_a1b2", true},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsRevokedAPIKey(tc.in); got != tc.want {
				t.Errorf("IsRevokedAPIKey(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestDeriveSlug(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"short", "acme", "acme"},
		{"spaces to dashes", "Big Corp", "big-corp"},
		{"lowercased", "ACME", "acme"},
		{"capped at 8", "zippy-labs", "zippy-la"},
		{"trailing dash trimmed", "abcdefg h", "abcdefg"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveSlug(tc.in); got != tc.want {
				t.Errorf("deriveSlug(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

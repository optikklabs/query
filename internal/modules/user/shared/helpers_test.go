package shared

import (
	"regexp"
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	for _, tc := range []struct {
		raw, want string
		ok        bool
	}{
		{"  Ann@Example.COM ", "ann@example.com", true},
		{"ann@example", "ann@example", true},
		{"no-at-sign", "no-at-sign", false},
		{"Ann <ann@example.com>", "ann <ann@example.com>", false},
		{"", "", false},
	} {
		got, ok := NormalizeEmail(tc.raw)
		if got != tc.want || ok != tc.ok {
			t.Errorf("NormalizeEmail(%q) = (%q, %v), want (%q, %v)", tc.raw, got, ok, tc.want, tc.ok)
		}
	}
}

func TestGeneratedSecrets(t *testing.T) {
	if !regexp.MustCompile(`^[A-Z2-9]{4}-[A-Z2-9]{4}$`).MatchString(GenerateUserCode()) {
		t.Fatal("user code has the wrong shape")
	}
	key := GenerateAPIKey()
	if !strings.HasPrefix(key, "ok_") || len(key) != 3+64 || APIKeyPrefix(key) != key[:11] {
		t.Fatalf("api key %q", key)
	}
	if a, b := GenerateSecretToken(), GenerateSecretToken(); a == b || len(a) != 64 {
		t.Fatalf("secret tokens %q, %q", a, b)
	}
	if APIKeyPrefix("short") != "short" {
		t.Fatal("prefix of a short key")
	}
}

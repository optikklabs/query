package shared

import "testing"

func TestPasswordIsValidRequiresStoredHash(t *testing.T) {
	if PasswordIsValid(nil, "password") {
		t.Fatal("a missing password hash must never authenticate")
	}
	empty := ""
	if PasswordIsValid(&empty, "password") {
		t.Fatal("an empty password hash must never authenticate")
	}
}

func TestPasswordIsValidUsesExactPassword(t *testing.T) {
	hash, err := HashPassword(" password ")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !PasswordIsValid(&hash, " password ") {
		t.Fatal("expected exact password to authenticate")
	}
	if PasswordIsValid(&hash, "password") {
		t.Fatal("password whitespace must not be discarded")
	}
}

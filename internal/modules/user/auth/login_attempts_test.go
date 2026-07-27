package auth

import (
	"fmt"
	"testing"
	"time"
)

func TestLoginAttemptsAreBounded(t *testing.T) {
	attempts := &loginAttempts{entries: make(map[string]attempt)}
	for i := 0; i < maxLoginAttemptEntries+100; i++ {
		attempts.fail(fmt.Sprintf("user-%d@example.com", i), "192.0.2.1")
	}
	if got := len(attempts.entries); got > maxLoginAttemptEntries {
		t.Fatalf("entries = %d, want at most %d", got, maxLoginAttemptEntries)
	}
}

func TestLoginAttemptsExpireIdleEntries(t *testing.T) {
	attempts := &loginAttempts{entries: map[string]attempt{}}
	key := attempts.key("user@example.com", "192.0.2.1")
	attempts.entries[key] = attempt{
		failures: 1,
		lastSeen: time.Now().Add(-loginAttemptTTL - time.Second),
	}

	if !attempts.allow("user@example.com", "192.0.2.1") {
		t.Fatal("expired attempt should be allowed")
	}
	if _, ok := attempts.entries[key]; ok {
		t.Fatal("expired attempt was not removed")
	}
}

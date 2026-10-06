package shared

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// randomHex returns n cryptographically random bytes, hex encoded.
// crypto/rand.Read never fails; it aborts the program instead.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateAPIKey returns a new tenant API key.
func GenerateAPIKey() string { return "ok_" + randomHex(32) }

// GenerateRevokedKey returns an unusable key that replaces a revoked one, so
// the stored hash never matches a key anyone holds.
func GenerateRevokedKey() string { return "revoked_" + randomHex(32) }

// GenerateSecretToken returns a high-entropy token for device codes and email
// verification links.
func GenerateSecretToken() string { return randomHex(32) }

// APIKeyPrefix is the non-secret prefix shown to identify an API key.
func APIKeyPrefix(raw string) string {
	const n = 11
	return raw[:min(len(raw), n)]
}

// userCodeAlphabet omits look-alike characters. Its length divides 256, so
// mapping random bytes onto it is unbiased.
const userCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GenerateUserCode returns the short code a user types to approve a device,
// formatted as XXXX-XXXX.
func GenerateUserCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = userCodeAlphabet[int(b[i])%len(userCodeAlphabet)]
	}
	return string(b[:4]) + "-" + string(b[4:])
}

// NullableString returns nil for a blank string, for optional columns.
func NullableString(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// NormalizeEmail trims and lowercases an address and reports whether it is
// a bare, well-formed email address.
func NormalizeEmail(raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	addr, err := mail.ParseAddress(email)
	return email, err == nil && addr.Address == email
}

// IsDuplicateEntry reports whether err is a MySQL unique-key violation.
func IsDuplicateEntry(err error) bool {
	const duplicateEntry = 1062
	me, ok := errors.AsType[*mysql.MySQLError](err)
	return ok && me.Number == duplicateEntry
}

package shared

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// "ok_" prefix enables secret-scanning; 32 bytes gives 256-bit entropy.
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ok_" + hex.EncodeToString(bytes), nil
}

// HashAPIKey is the stored form of an API key: hex SHA-256. Keys are
// 256-bit random values, so a fast unsalted hash is safe and keeps the
// ingest hot-path lookup cheap. Ingest's authrepo mirrors this hashing.
func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// APIKeyPrefix is the display fragment stored beside the hash so the UI
// can identify a key without being able to recover it.
func APIKeyPrefix(raw string) string {
	const n = 11 // "ok_" + 8 hex chars
	if len(raw) < n {
		return raw
	}
	return raw[:n]
}

// revokedKeyPrefix marks a key the client can no longer use. api_key_hash is
// NOT NULL UNIQUE, so revoke writes a random unique sentinel (not a blank)
// that no client holds, disabling ingest until the tenant rotates a fresh key.
const revokedKeyPrefix = "revoked_"

func GenerateRevokedKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return revokedKeyPrefix + hex.EncodeToString(bytes), nil
}

// userCodeAlphabet excludes ambiguous chars (0/O, 1/I) for typo-free entry.
const userCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GenerateDeviceCode is the secret the CLI polls with: 256-bit hex.
func GenerateDeviceCode() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateUserCode is the short human code shown to the user as XXXX-XXXX.
func GenerateUserCode() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	chars := make([]byte, 8)
	for i, b := range bytes {
		chars[i] = userCodeAlphabet[int(b)%len(userCodeAlphabet)]
	}
	return string(chars[:4]) + "-" + string(chars[4:]), nil
}

func NullableString(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

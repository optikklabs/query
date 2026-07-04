package user

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// ParseTeamMemberships parses the json string of memberships.
func ParseTeamMemberships(raw string) ([]TeamMembership, error) {
	var memberships []TeamMembership
	if raw == "" || raw == "null" {
		return memberships, nil
	}
	err := json.Unmarshal([]byte(raw), &memberships)
	return memberships, err
}

func BuildTeamMembershipsJSON(memberships []TeamMembership) (string, error) {
	if len(memberships) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(memberships)
	return string(data), err
}

func TeamIDsFromMemberships(memberships []TeamMembership) []int64 {
	ids := make([]int64, len(memberships))
	for i, membership := range memberships {
		ids[i] = membership.TeamID
	}
	return ids
}

// "ok_" prefix enables secret-scanning; 32 bytes gives 256-bit entropy.
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ok_" + hex.EncodeToString(bytes), nil
}

// revokedKeyPrefix marks a key the client can no longer use. api_key is NOT NULL
// UNIQUE, so revoke writes a random unique sentinel (not a blank) that no client
// holds, disabling ingest until the team rotates a fresh key.
const revokedKeyPrefix = "revoked_"

func GenerateRevokedKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return revokedKeyPrefix + hex.EncodeToString(bytes), nil
}

// IsRevokedAPIKey reports whether a key is a revoke sentinel (no live key).
func IsRevokedAPIKey(key string) bool {
	return strings.HasPrefix(key, revokedKeyPrefix)
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

func ValueOr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}

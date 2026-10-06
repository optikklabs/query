package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

const refreshTokenBytes = 32

// GenerateRefreshToken returns a new refresh token and its storage hash.
func GenerateRefreshToken() (raw, hash string) {
	buf := make([]byte, refreshTokenBytes)
	_, _ = rand.Read(buf) // never fails; aborts the program instead
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashSecret(raw)
}

// HashSecret is the storage hash for high-entropy secrets: refresh tokens,
// API keys and verification tokens. Their entropy makes a fast unsalted
// hash sufficient.
func HashSecret(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func NewFamilyID() string {
	return uuid.NewString()
}

func (s *Service) RefreshTTL() time.Duration {
	return s.refreshTTL
}

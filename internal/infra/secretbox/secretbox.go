// Package secretbox provides authenticated symmetric encryption (AES-256-GCM)
// for at-rest secrets such as tenant BYO provider API keys. The key is supplied
// once at boot; ciphertext and its per-message nonce are stored separately so
// the column layout mirrors the crypto primitives.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// keyLen is the AES-256 key size in bytes.
const keyLen = 32

// Box seals and opens secrets under a single AES-256-GCM key.
type Box struct {
	gcm cipher.AEAD
}

// New builds a Box from a base64-encoded 32-byte key. It returns an error when
// the key is absent or the wrong size, so callers can gate features on presence.
func New(base64Key string) (*Box, error) {
	if base64Key == "" {
		return nil, errors.New("secretbox: encryption key is not configured")
	}
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("secretbox: key is not valid base64: %w", err)
	}
	if len(key) != keyLen {
		return nil, fmt.Errorf("secretbox: key must be %d bytes, got %d", keyLen, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secretbox: cannot init cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secretbox: cannot init GCM: %w", err)
	}
	return &Box{gcm: gcm}, nil
}

// Seal encrypts plaintext, returning the ciphertext and the fresh random nonce
// used. The nonce must be persisted alongside the ciphertext for Open.
func (b *Box) Seal(plaintext []byte) (ciphertext, nonce []byte, err error) {
	nonce = make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("secretbox: cannot generate nonce: %w", err)
	}
	ciphertext = b.gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// Open decrypts ciphertext with its stored nonce.
func (b *Box) Open(ciphertext, nonce []byte) ([]byte, error) {
	if len(nonce) != b.gcm.NonceSize() {
		return nil, errors.New("secretbox: nonce size mismatch")
	}
	plaintext, err := b.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("secretbox: decryption failed: %w", err)
	}
	return plaintext, nil
}

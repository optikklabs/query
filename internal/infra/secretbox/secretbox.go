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

const keyLen = 32

type Box struct {
	gcm cipher.AEAD
}

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

func (b *Box) Seal(plaintext []byte) (ciphertext, nonce []byte, err error) {
	nonce = make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("secretbox: cannot generate nonce: %w", err)
	}
	ciphertext = b.gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

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

package secretbox

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func testKey() string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, keyLen))
}

func TestSealOpenRoundTrip(t *testing.T) {
	box, err := New(testKey())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	secret := []byte("sk-test-1234567890")
	ct, nonce, err := box.Seal(secret)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Equal(ct, secret) {
		t.Fatal("ciphertext must differ from plaintext")
	}
	got, err := box.Open(ct, nonce)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(got, secret) {
		t.Fatalf("round trip mismatch: got %q", got)
	}
}

func TestSealUsesFreshNonce(t *testing.T) {
	box, _ := New(testKey())
	_, n1, _ := box.Seal([]byte("a"))
	_, n2, _ := box.Seal([]byte("a"))
	if bytes.Equal(n1, n2) {
		t.Fatal("nonces must be unique per Seal")
	}
}

func TestOpenRejectsTamper(t *testing.T) {
	box, _ := New(testKey())
	ct, nonce, _ := box.Seal([]byte("secret"))
	ct[0] ^= 0xFF
	if _, err := box.Open(ct, nonce); err == nil {
		t.Fatal("tampered ciphertext must fail authentication")
	}
}

func TestNewRejectsBadKey(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("empty key must error")
	}
	if _, err := New(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("short key must error")
	}
}

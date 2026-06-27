package token

import "testing"

func TestGenerateRefreshTokenIsUniqueAndHashed(t *testing.T) {
	raw1, hash1, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	raw2, hash2, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	if raw1 == "" || hash1 == "" {
		t.Fatal("expected non-empty token and hash")
	}
	if raw1 == raw2 {
		t.Fatal("expected distinct raw tokens")
	}
	if hash1 == raw1 {
		t.Fatal("hash must not equal the raw token")
	}
	if len(hash1) != 64 {
		t.Fatalf("expected 64-char sha256 hex, got %d", len(hash1))
	}
	if hash1 == hash2 {
		t.Fatal("expected distinct hashes")
	}
}

func TestHashRefreshTokenIsDeterministic(t *testing.T) {
	raw, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if got := HashRefreshToken(raw); got != hash {
		t.Fatalf("HashRefreshToken mismatch: got %s want %s", got, hash)
	}
}

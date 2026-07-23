package providerkeys

import "time"

// ProviderKey is the safe projection of a stored key: never the plaintext.
type ProviderKey struct {
	ID        int64     `json:"id"`
	Provider  string    `json:"provider"`
	Label     string    `json:"label"`
	Last4     string    `json:"last4"`
	CreatedAt time.Time `json:"createdAt"`
}

// CreateRequest registers a new BYO provider key. The plaintext is encrypted at
// rest and never echoed back.
type CreateRequest struct {
	Provider string `json:"provider"`
	Label    string `json:"label"`
	APIKey   string `json:"apiKey"`
}

type keyRow struct {
	ID        int64     `db:"id"`
	Provider  string    `db:"provider"`
	Label     string    `db:"label"`
	Last4     string    `db:"key_last4"`
	CreatedAt time.Time `db:"created_at"`
}

// secretRow carries the ciphertext for internal decryption only.
type secretRow struct {
	Ciphertext []byte `db:"key_ciphertext"`
	Nonce      []byte `db:"nonce"`
}

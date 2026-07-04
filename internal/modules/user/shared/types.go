package shared

import (
	"time"
)


type AuthUser struct {
	ID           int64   `db:"id"`
	Email        string  `db:"email"`
	PasswordHash *string `db:"password_hash"`
	Name         string  `db:"name"`
	TenantID     int64   `db:"tenant_id"`
	IsAdmin      bool    `db:"is_admin"`
}

type UserRecord struct {
	ID          int64      `db:"id"`
	Email       string     `db:"email"`
	Name        string     `db:"name"`
	TenantID    int64      `db:"tenant_id"`
	Active      bool       `db:"active"`
	CreatedAt   time.Time  `db:"created_at"`
}

type RefreshTokenRecord struct {
	ID        int64      `db:"id"`
	UserID    int64      `db:"user_id"`
	FamilyID  string     `db:"family_id"`
	TokenHash string     `db:"token_hash"`
	ExpiresAt time.Time  `db:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at"`
	CreatedAt time.Time  `db:"created_at"`
}

type DeviceCodeRecord struct {
	ID           int64      `db:"id"`
	DeviceCode   string     `db:"device_code"`
	UserCode     string     `db:"user_code"`
	UserID       *int64     `db:"user_id"`
	ApprovedAt   *time.Time `db:"approved_at"`
	ConsumedAt   *time.Time `db:"consumed_at"`
	LastPolledAt *time.Time `db:"last_polled_at"`
	ExpiresAt    time.Time  `db:"expires_at"`
	CreatedAt    time.Time  `db:"created_at"`
}

type TenantRecord struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	Active    bool      `db:"active"`
	APIKey    string    `db:"api_key"`
	CreatedAt time.Time `db:"created_at"`
}

// MessageResponse is a generic single-message API response.
type MessageResponse struct {
	Message string `json:"message"`
}

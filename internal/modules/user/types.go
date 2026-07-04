package user

import (
	"time"
)

// TeamMembership represents user's role in a team.
type TeamMembership struct {
	TeamID int64  `json:"team_id"`
	Role   string `json:"role"`
}

type AuthUser struct {
	ID           int64   `db:"id"`
	Email        string  `db:"email"`
	PasswordHash *string `db:"password_hash"`
	Name         string  `db:"name"`
	AvatarURL    *string `db:"avatar_url"`
	TeamsJSON    *string `db:"teams"`
	IsAdmin      bool    `db:"is_admin"`
}

type UserRecord struct {
	ID          int64      `db:"id"`
	Email       string     `db:"email"`
	Name        string     `db:"name"`
	AvatarURL   *string    `db:"avatar_url"`
	TeamsJSON   *string    `db:"teams"`
	Active      bool       `db:"active"`
	LastLoginAt *time.Time `db:"last_login_at"`
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

type TeamRecord struct {
	ID          int64     `db:"id"`
	OrgName     string    `db:"org_name"`
	Name        string    `db:"name"`
	Slug        string    `db:"slug"`
	Description *string   `db:"description"`
	Active      bool      `db:"active"`
	Color       string    `db:"color"`
	Icon        *string   `db:"icon"`
	APIKey      string    `db:"api_key"`
	CreatedAt   time.Time `db:"created_at"`
}

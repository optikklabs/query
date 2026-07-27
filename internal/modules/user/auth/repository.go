package auth

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/user/shared"
)

// Repository holds the auth-related MySQL access (users, refresh tokens, tenants).
type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql")}
}

func (r *Repository) FindActiveUserByEmail(ctx context.Context, email string) (shared.AuthUser, error) {
	var u shared.AuthUser
	err := dbutil.GetSQL(ctx, r.db, "user.FindActiveUserByEmail", &u, `
		SELECT id, email, password_hash, name, tenant_id, role
		FROM users
		WHERE email = ? AND active = 1
		LIMIT 1
	`, strings.TrimSpace(email))
	return u, err
}

func (r *Repository) FindActiveUserByID(ctx context.Context, userID int64) (shared.UserRecord, error) {
	var u shared.UserRecord
	err := dbutil.GetSQL(ctx, r.db, "user.FindActiveUserByID", &u, `
		SELECT id, email, name, tenant_id, active, role, created_at
		FROM users
		WHERE id = ? AND active = 1
		LIMIT 1
	`, userID)
	return u, err
}

func (r *Repository) FindAuthUserByID(ctx context.Context, userID int64) (shared.AuthUser, error) {
	var u shared.AuthUser
	err := dbutil.GetSQL(ctx, r.db, "user.FindAuthUserByID", &u, `
		SELECT id, email, password_hash, name, tenant_id, role
		FROM users
		WHERE id = ? AND active = 1
		LIMIT 1
	`, userID)
	return u, err
}

func (r *Repository) UpdatePasswordAndRevokeSessions(ctx context.Context, userID int64, passwordHash string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE refresh_tokens SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL
	`, time.Now().UTC(), userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) InsertRefreshToken(ctx context.Context, userID int64, familyID, tokenHash string, expiresAt time.Time) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.InsertRefreshToken", `
		INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at)
		VALUES (?, ?, ?, ?)
	`, userID, familyID, tokenHash, expiresAt)
	return err
}

func (r *Repository) FindRefreshTokenByHash(ctx context.Context, tokenHash string) (shared.RefreshTokenRecord, error) {
	var t shared.RefreshTokenRecord
	err := dbutil.GetSQL(ctx, r.db, "user.FindRefreshTokenByHash", &t, `
		SELECT id, user_id, family_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = ?
		LIMIT 1
	`, tokenHash)
	return t, err
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.RevokeRefreshToken", `
		UPDATE refresh_tokens SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL
	`, time.Now().UTC(), tokenHash)
	return err
}

// RotateRefreshToken consumes the current token and creates its replacement in
// one transaction. The conditional update prevents concurrent refreshes from
// minting more than one successor.
func (r *Repository) RotateRefreshToken(ctx context.Context, currentHash string, userID int64, familyID, newHash string, expiresAt time.Time) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = ?
		WHERE token_hash = ? AND revoked_at IS NULL AND expires_at > ?
	`, now, currentHash, now)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated != 1 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at)
		VALUES (?, ?, ?, ?)
	`, userID, familyID, newHash, expiresAt); err != nil {
		return err
	}
	return tx.Commit()
}

// FindTenantByID loads a tenant regardless of active state so login can tell a
// suspended (trial-expired) tenant apart from a genuinely missing one.
func (r *Repository) FindTenantByID(ctx context.Context, tenantID int64) (shared.TenantRecord, error) {
	var t shared.TenantRecord
	err := dbutil.GetSQL(ctx, r.db, "user.FindTenantByID", &t, `
		SELECT id, name, active, api_key_prefix, account_status, trial_ends_at, created_at
		FROM tenant
		WHERE id = ?
		LIMIT 1
	`, tenantID)
	return t, err
}

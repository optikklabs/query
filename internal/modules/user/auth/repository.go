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

func (r *Repository) FindActiveUserByEmail(email string) (shared.AuthUser, error) {
	var u shared.AuthUser
	err := dbutil.GetSQL(context.Background(), r.db, "user.FindActiveUserByEmail", &u, `
		SELECT id, email, password_hash, name, tenant_id, is_admin
		FROM users
		WHERE email = ? AND active = 1
		LIMIT 1
	`, strings.TrimSpace(email))
	return u, err
}

func (r *Repository) FindActiveUserByID(userID int64) (shared.UserRecord, error) {
	var u shared.UserRecord
	err := dbutil.GetSQL(context.Background(), r.db, "user.FindActiveUserByID", &u, `
		SELECT id, email, name, tenant_id, active, created_at
		FROM users
		WHERE id = ? AND active = 1
		LIMIT 1
	`, userID)
	return u, err
}


func (r *Repository) InsertRefreshToken(userID int64, familyID, tokenHash string, expiresAt time.Time) error {
	_, err := dbutil.ExecSQL(context.Background(), r.db, "user.InsertRefreshToken", `
		INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at)
		VALUES (?, ?, ?, ?)
	`, userID, familyID, tokenHash, expiresAt)
	return err
}

func (r *Repository) FindRefreshTokenByHash(tokenHash string) (shared.RefreshTokenRecord, error) {
	var t shared.RefreshTokenRecord
	err := dbutil.GetSQL(context.Background(), r.db, "user.FindRefreshTokenByHash", &t, `
		SELECT id, user_id, family_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = ?
		LIMIT 1
	`, tokenHash)
	return t, err
}

func (r *Repository) RevokeRefreshToken(tokenHash string) error {
	_, err := dbutil.ExecSQL(context.Background(), r.db, "user.RevokeRefreshToken", `
		UPDATE refresh_tokens SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL
	`, time.Now().UTC(), tokenHash)
	return err
}

func (r *Repository) RevokeRefreshTokenFamily(familyID string) error {
	_, err := dbutil.ExecSQL(context.Background(), r.db, "user.RevokeRefreshTokenFamily", `
		UPDATE refresh_tokens SET revoked_at = ? WHERE family_id = ? AND revoked_at IS NULL
	`, time.Now().UTC(), familyID)
	return err
}

func (r *Repository) ListActiveTenantsByIDs(tenantIDs []int64) ([]shared.TenantRecord, error) {
	if len(tenantIDs) == 0 {
		return []shared.TenantRecord{}, nil
	}
	query, args, err := sqlx.In(`
		SELECT id, name, active, api_key, created_at
		FROM tenant
		WHERE id IN (?) AND active = 1
		ORDER BY created_at DESC
	`, tenantIDs)
	if err != nil {
		return nil, err
	}
	query = r.db.Rebind(query)
	var records []shared.TenantRecord
	if err := dbutil.SelectSQL(context.Background(), r.db, "user.ListActiveTenantsByIDs", &records, query, args...); err != nil {
		return nil, err
	}
	return records, nil
}


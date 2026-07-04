package users

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/user/shared"
)

// Repository holds the user-provisioning MySQL access.
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

func (r *Repository) FindUserByID(userID int64) (shared.UserRecord, error) {
	var u shared.UserRecord
	err := dbutil.GetSQL(context.Background(), r.db, "user.FindUserByID", &u, `
		SELECT id, email, name, tenant_id, active, created_at
		FROM users
		WHERE id = ?
		LIMIT 1
	`, userID)
	return u, err
}

func (r *Repository) CreateUser(email, passwordHash, name string, tenantID int64, isAdmin bool, createdAt time.Time) (int64, error) {
	res, err := dbutil.ExecSQL(context.Background(), r.db, "user.CreateUser", `
		INSERT INTO users (email, password_hash, name, tenant_id, active, is_admin, created_at)
		VALUES (?, ?, ?, ?, 1, ?, ?)
	`, email, shared.NullableString(passwordHash), name, tenantID, isAdmin, createdAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}


// ListUsersByTenantID finds all active users belonging to the given tenant ID.
func (r *Repository) ListUsersByTenantID(tenantID int64) ([]shared.UserRecord, error) {
	var records []shared.UserRecord
	err := dbutil.SelectSQL(context.Background(), r.db, "user.ListUsersByTenantID", &records, `
		SELECT id, email, name, tenant_id, active, created_at
		FROM users
		WHERE active = 1 AND tenant_id = ?
		ORDER BY created_at DESC
	`, tenantID)
	return records, err
}

// DeactivateUser soft-deletes a user by setting active = 0.
func (r *Repository) DeactivateUser(userID int64) error {
	_, err := dbutil.ExecSQL(context.Background(), r.db, "user.DeactivateUser", `
		UPDATE users SET active = 0 WHERE id = ?
	`, userID)
	return err
}

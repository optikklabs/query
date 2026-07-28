package users

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/user/shared"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql")}
}

func (r *Repository) FindUserByID(ctx context.Context, userID, tenantID int64) (shared.UserRecord, error) {
	var u shared.UserRecord
	err := dbutil.GetSQL(ctx, r.db, "user.FindUserByID", &u, `
		SELECT id, email, name, tenant_id, active, role, created_at
		FROM users
		WHERE id = ? AND tenant_id = ? AND active = 1
		LIMIT 1
	`, userID, tenantID)
	return u, err
}

func (r *Repository) CreateUser(ctx context.Context, email, passwordHash, name string, tenantID int64, role string, createdAt time.Time) (int64, error) {
	res, err := dbutil.ExecSQL(ctx, r.db, "user.CreateUser", `
		INSERT INTO users (email, password_hash, name, tenant_id, active, role, created_at)
		VALUES (?, ?, ?, ?, 1, ?, ?)
	`, email, shared.NullableString(passwordHash), name, tenantID, role, createdAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repository) ListUsersByTenantID(ctx context.Context, tenantID int64) ([]shared.UserRecord, error) {
	var records []shared.UserRecord
	err := dbutil.SelectSQL(ctx, r.db, "user.ListUsersByTenantID", &records, `
		SELECT id, email, name, tenant_id, active, role, created_at
		FROM users
		WHERE active = 1 AND tenant_id = ?
		ORDER BY created_at DESC
	`, tenantID)
	return records, err
}

func (r *Repository) UpdateUserRole(ctx context.Context, userID, tenantID int64, role string) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.UpdateUserRole", `
		UPDATE users SET role = ? WHERE id = ? AND tenant_id = ? AND active = 1
	`, role, userID, tenantID)
	return err
}

func (r *Repository) CountActiveAdmins(ctx context.Context, tenantID int64) (int, error) {
	var n int
	err := dbutil.GetSQL(ctx, r.db, "user.CountActiveAdmins", &n, `
		SELECT COUNT(*) FROM users
		WHERE tenant_id = ? AND active = 1 AND role = 'admin'
	`, tenantID)
	return n, err
}

func (r *Repository) DeactivateUser(ctx context.Context, userID, tenantID int64) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.DeactivateUser", `
		UPDATE users SET active = 0 WHERE id = ? AND tenant_id = ?
	`, userID, tenantID)
	return err
}

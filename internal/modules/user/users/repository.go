package users

import (
	"context"
	"database/sql"
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

// FindUserByID loads an active user scoped to a tenant. Tenant scoping prevents
// an admin from acting on users outside their own org.
func (r *Repository) FindUserByID(userID, tenantID int64) (shared.UserRecord, error) {
	var u shared.UserRecord
	err := dbutil.GetSQL(context.Background(), r.db, "user.FindUserByID", &u, `
		SELECT id, email, name, tenant_id, active, role, created_at
		FROM users
		WHERE id = ? AND tenant_id = ? AND active = 1
		LIMIT 1
	`, userID, tenantID)
	return u, err
}

func (r *Repository) CreateUser(email, passwordHash, name string, tenantID int64, role string, createdAt time.Time) (int64, error) {
	res, err := dbutil.ExecSQL(context.Background(), r.db, "user.CreateUser", `
		INSERT INTO users (email, password_hash, name, tenant_id, active, role, created_at)
		VALUES (?, ?, ?, ?, 1, ?, ?)
	`, email, shared.NullableString(passwordHash), name, tenantID, role, createdAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListUsersByTenantID finds all active users belonging to the given tenant ID.
func (r *Repository) ListUsersByTenantID(tenantID int64) ([]shared.UserRecord, error) {
	var records []shared.UserRecord
	err := dbutil.SelectSQL(context.Background(), r.db, "user.ListUsersByTenantID", &records, `
		SELECT id, email, name, tenant_id, active, role, created_at
		FROM users
		WHERE active = 1 AND tenant_id = ?
		ORDER BY created_at DESC
	`, tenantID)
	return records, err
}

// UpdateUserRole sets an active user's role within a tenant.
func (r *Repository) UpdateUserRole(userID, tenantID int64, role string) error {
	_, err := dbutil.ExecSQL(context.Background(), r.db, "user.UpdateUserRole", `
		UPDATE users SET role = ? WHERE id = ? AND tenant_id = ? AND active = 1
	`, role, userID, tenantID)
	return err
}

// CountActiveAdmins reports how many active admins a tenant has, used to block
// removing or demoting the last admin.
func (r *Repository) CountActiveAdmins(tenantID int64) (int, error) {
	var n int
	err := dbutil.GetSQL(context.Background(), r.db, "user.CountActiveAdmins", &n, `
		SELECT COUNT(*) FROM users
		WHERE tenant_id = ? AND active = 1 AND role = 'admin'
	`, tenantID)
	return n, err
}

// DeactivateUser soft-deletes a user within a tenant by setting active = 0.
func (r *Repository) DeactivateUser(userID, tenantID int64) error {
	_, err := dbutil.ExecSQL(context.Background(), r.db, "user.DeactivateUser", `
		UPDATE users SET active = 0 WHERE id = ? AND tenant_id = ?
	`, userID, tenantID)
	return err
}

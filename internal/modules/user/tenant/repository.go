package tenant

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/user/shared"
)

// Repository holds the tenant MySQL access (lookup + api-key updates).
type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql")}
}

func (r *Repository) FindTenantByID(ctx context.Context, tenantID int64) (shared.TenantRecord, error) {
	var t shared.TenantRecord
	err := dbutil.GetSQL(ctx, r.db, "user.FindTenantByID", &t, `
		SELECT id, name, active, api_key_prefix, created_at
		FROM tenant
		WHERE id = ?
		LIMIT 1
	`, tenantID)
	return t, err
}

func (r *Repository) UpdateTenantAPIKey(ctx context.Context, tenantID int64, apiKeyHash, apiKeyPrefix string) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.UpdateTenantAPIKey", `
		UPDATE tenant SET api_key_hash = ?, api_key_prefix = ? WHERE id = ?
	`, apiKeyHash, apiKeyPrefix, tenantID)
	return err
}

func (r *Repository) DeactivateTenant(ctx context.Context, tenantID int64) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.DeactivateTenant", `
		UPDATE tenant SET active = 0 WHERE id = ?
	`, tenantID)
	return err
}

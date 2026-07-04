package signup

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// mysqlDuplicateEntry is ER_DUP_ENTRY, raised on a unique-key collision.
const mysqlDuplicateEntry = 1062

// Repository owns the atomic tenant + first-user provisioning for signup.
type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql")}
}

// CreateTenantWithAdmin inserts the tenant and its first admin user in one
// transaction so a duplicate email can never leave an orphan tenant. Uses a raw
// tx because the instrumented helpers only accept *sqlx.DB.
func (r *Repository) CreateTenantWithAdmin(
	ctx context.Context, tenantName, apiKey, email, passwordHash, userName string,
) (tenantID, userID int64, err error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO tenant (name, api_key) VALUES (?, ?)`, tenantName, apiKey)
	if err != nil {
		return 0, 0, err
	}
	if tenantID, err = res.LastInsertId(); err != nil {
		return 0, 0, err
	}

	res, err = tx.ExecContext(ctx, `
		INSERT INTO users (email, password_hash, name, tenant_id, active, is_admin, created_at)
		VALUES (?, ?, ?, ?, 1, 0, ?)
	`, email, passwordHash, userName, tenantID, time.Now().UTC())
	if err != nil {
		return 0, 0, err
	}
	if userID, err = res.LastInsertId(); err != nil {
		return 0, 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, err
	}
	return tenantID, userID, nil
}

// IsDuplicateEmail reports whether err is the unique-email collision. Tenant
// names are not unique, so the only 1062 signup can hit is the users email key.
func IsDuplicateEmail(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == mysqlDuplicateEntry
}

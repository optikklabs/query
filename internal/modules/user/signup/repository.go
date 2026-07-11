package signup

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/optikklabs/query/internal/modules/user/shared"
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
	ctx context.Context, tenantName, apiKey, email, passwordHash, userName, verificationHash string, verificationExpiry, trialEndsAt time.Time,
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

	// account_status/plan default to trialing/free; set the trial deadline.
	// Only the key's hash and display prefix are persisted.
	res, err := tx.ExecContext(ctx,
		`INSERT INTO tenant (name, api_key_hash, api_key_prefix, trial_ends_at) VALUES (?, ?, ?, ?)`,
		tenantName, shared.HashAPIKey(apiKey), shared.APIKeyPrefix(apiKey), trialEndsAt)
	if err != nil {
		return 0, 0, err
	}
	if tenantID, err = res.LastInsertId(); err != nil {
		return 0, 0, err
	}

	// The signup owner is the first admin of their new tenant.
	res, err = tx.ExecContext(ctx, `
		INSERT INTO users (email, password_hash, name, tenant_id, active, role, created_at)
		VALUES (?, ?, ?, ?, 0, 'admin', ?)
	`, email, passwordHash, userName, tenantID, time.Now().UTC())
	if err != nil {
		return 0, 0, err
	}
	if userID, err = res.LastInsertId(); err != nil {
		return 0, 0, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO email_verifications (user_id, token_hash, expires_at) VALUES (?, ?, ?)`, userID, verificationHash, verificationExpiry); err != nil {
		return 0, 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, err
	}
	return tenantID, userID, nil
}

func (r *Repository) ConsumeVerification(ctx context.Context, tokenHash string) (shared.AuthUser, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return shared.AuthUser{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var user shared.AuthUser
	err = tx.GetContext(ctx, &user, `SELECT u.id, u.email, u.password_hash, u.name, u.tenant_id, u.role FROM email_verifications v JOIN users u ON u.id=v.user_id WHERE v.token_hash=? AND v.consumed_at IS NULL AND v.expires_at > UTC_TIMESTAMP() FOR UPDATE`, tokenHash)
	if err != nil {
		return shared.AuthUser{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE email_verifications SET consumed_at=UTC_TIMESTAMP() WHERE token_hash=? AND consumed_at IS NULL`, tokenHash); err != nil {
		return shared.AuthUser{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET active=1 WHERE id=?`, user.ID); err != nil {
		return shared.AuthUser{}, err
	}
	if err = tx.Commit(); err != nil {
		return shared.AuthUser{}, err
	}
	return user, nil
}

func (r *Repository) RotateTenantAPIKey(ctx context.Context, tenantID int64, apiKey string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tenant SET api_key_hash=?, api_key_prefix=? WHERE id=?`, shared.HashAPIKey(apiKey), shared.APIKeyPrefix(apiKey), tenantID)
	return err
}

// IsDuplicateEmail reports whether err is the unique-email collision. Tenant
// names are not unique, so the only 1062 signup can hit is the users email key.
func IsDuplicateEmail(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == mysqlDuplicateEntry
}

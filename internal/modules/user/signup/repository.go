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

type tenantAdminSignup struct {
	TenantName         string
	APIKey             string
	Email              string
	PasswordHash       string
	UserName           string
	Active             bool
	VerificationHash   string
	VerificationExpiry time.Time
	TrialEndsAt        time.Time
	TermsAcceptedAt    time.Time
	TermsVersion       string
}

// CreateTenantWithAdmin inserts the tenant and its first admin user in one
// transaction so a duplicate email can never leave an orphan tenant. Uses a raw
// tx because the instrumented helpers only accept *sqlx.DB.
func (r *Repository) CreateTenantWithAdmin(ctx context.Context, signup tenantAdminSignup) (user shared.AuthUser, err error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return shared.AuthUser{}, err
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
		signup.TenantName, shared.HashAPIKey(signup.APIKey), shared.APIKeyPrefix(signup.APIKey), signup.TrialEndsAt)
	if err != nil {
		return shared.AuthUser{}, err
	}
	var tenantID int64
	if tenantID, err = res.LastInsertId(); err != nil {
		return shared.AuthUser{}, err
	}

	// The signup owner is the first admin of their new tenant.
	res, err = tx.ExecContext(ctx, `
		INSERT INTO users (email, password_hash, name, tenant_id, active, role, created_at, terms_accepted_at, terms_version)
		VALUES (?, ?, ?, ?, ?, 'admin', ?, ?, ?)
	`, signup.Email, signup.PasswordHash, signup.UserName, tenantID, signup.Active, time.Now().UTC(), signup.TermsAcceptedAt, signup.TermsVersion)
	if err != nil {
		return shared.AuthUser{}, err
	}
	var userID int64
	if userID, err = res.LastInsertId(); err != nil {
		return shared.AuthUser{}, err
	}
	if !signup.Active {
		if _, err = tx.ExecContext(ctx, `INSERT INTO email_verifications (user_id, token_hash, expires_at) VALUES (?, ?, ?)`, userID, signup.VerificationHash, signup.VerificationExpiry); err != nil {
			return shared.AuthUser{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return shared.AuthUser{}, err
	}
	return shared.AuthUser{
		ID:           userID,
		Email:        signup.Email,
		PasswordHash: &signup.PasswordHash,
		Name:         signup.UserName,
		TenantID:     tenantID,
		Role:         shared.RoleAdmin,
	}, nil
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

var ErrAlreadyVerified = errors.New("user is already verified")

// UpdateUnverifiedTenantAndAdmin updates an existing but unverified user and their tenant in place.
// This allows users to retry signing up if their verification email failed to send.
func (r *Repository) UpdateUnverifiedTenantAndAdmin(ctx context.Context, signup tenantAdminSignup) (user shared.AuthUser, err error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return shared.AuthUser{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var uID, tID int64
	err = tx.QueryRowContext(ctx, `SELECT id, tenant_id FROM users WHERE email=? AND active=0 FOR UPDATE`, signup.Email).Scan(&uID, &tID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Either user doesn't exist (shouldn't happen here) or active=1
			return shared.AuthUser{}, ErrAlreadyVerified
		}
		return shared.AuthUser{}, err
	}

	// Update user. Retry signups stay inactive when verification is required,
	// or become active immediately when the verification gate is disabled.
	if _, err = tx.ExecContext(ctx, `
		UPDATE users SET password_hash=?, name=?, active=?, terms_accepted_at=?, terms_version=? WHERE id=?
	`, signup.PasswordHash, signup.UserName, signup.Active, signup.TermsAcceptedAt, signup.TermsVersion, uID); err != nil {
		return shared.AuthUser{}, err
	}

	// Update tenant
	if _, err = tx.ExecContext(ctx, `
		UPDATE tenant SET name=?, api_key_hash=?, api_key_prefix=?, trial_ends_at=? WHERE id=?
	`, signup.TenantName, shared.HashAPIKey(signup.APIKey), shared.APIKeyPrefix(signup.APIKey), signup.TrialEndsAt, tID); err != nil {
		return shared.AuthUser{}, err
	}

	// Replace or clear any stale verification token.
	if _, err = tx.ExecContext(ctx, `DELETE FROM email_verifications WHERE user_id=?`, uID); err != nil {
		return shared.AuthUser{}, err
	}
	if !signup.Active {
		if _, err = tx.ExecContext(ctx, `INSERT INTO email_verifications (user_id, token_hash, expires_at) VALUES (?, ?, ?)`, uID, signup.VerificationHash, signup.VerificationExpiry); err != nil {
			return shared.AuthUser{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return shared.AuthUser{}, err
	}
	return shared.AuthUser{
		ID:           uID,
		Email:        signup.Email,
		PasswordHash: &signup.PasswordHash,
		Name:         signup.UserName,
		TenantID:     tID,
		Role:         shared.RoleAdmin,
	}, nil
}

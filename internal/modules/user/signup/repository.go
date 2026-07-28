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

const mysqlDuplicateEntry = 1062

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

func IsDuplicateEmail(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == mysqlDuplicateEntry
}

var ErrAlreadyVerified = errors.New("user is already verified")

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

			return shared.AuthUser{}, ErrAlreadyVerified
		}
		return shared.AuthUser{}, err
	}

	if _, err = tx.ExecContext(ctx, `
		UPDATE users SET password_hash=?, name=?, active=?, terms_accepted_at=?, terms_version=? WHERE id=?
	`, signup.PasswordHash, signup.UserName, signup.Active, signup.TermsAcceptedAt, signup.TermsVersion, uID); err != nil {
		return shared.AuthUser{}, err
	}

	if _, err = tx.ExecContext(ctx, `
		UPDATE tenant SET name=?, api_key_hash=?, api_key_prefix=?, trial_ends_at=? WHERE id=?
	`, signup.TenantName, shared.HashAPIKey(signup.APIKey), shared.APIKeyPrefix(signup.APIKey), signup.TrialEndsAt, tID); err != nil {
		return shared.AuthUser{}, err
	}

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

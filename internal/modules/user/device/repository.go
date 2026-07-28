package device

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

func (r *Repository) InsertDeviceCode(ctx context.Context, deviceCode, userCode string, expiresAt time.Time) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.InsertDeviceCode", `
		INSERT INTO device_codes (device_code, user_code, expires_at)
		VALUES (?, ?, ?)
	`, deviceCode, userCode, expiresAt)
	return err
}

func (r *Repository) FindDeviceCode(ctx context.Context, deviceCode string) (shared.DeviceCodeRecord, error) {
	var d shared.DeviceCodeRecord
	err := dbutil.GetSQL(ctx, r.db, "user.FindDeviceCode", &d, `
		SELECT id, device_code, user_code, user_id, approved_at, consumed_at, last_polled_at, expires_at, created_at
		FROM device_codes
		WHERE device_code = ?
		LIMIT 1
	`, deviceCode)
	return d, err
}

func (r *Repository) FindDeviceCodeByUserCode(ctx context.Context, userCode string) (shared.DeviceCodeRecord, error) {
	var d shared.DeviceCodeRecord
	err := dbutil.GetSQL(ctx, r.db, "user.FindDeviceCodeByUserCode", &d, `
		SELECT id, device_code, user_code, user_id, approved_at, consumed_at, last_polled_at, expires_at, created_at
		FROM device_codes
		WHERE user_code = ?
		LIMIT 1
	`, userCode)
	return d, err
}

func (r *Repository) TouchDeviceCodePolled(ctx context.Context, deviceCode string, at time.Time) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.TouchDeviceCodePolled", `
		UPDATE device_codes SET last_polled_at = ? WHERE device_code = ?
	`, at, deviceCode)
	return err
}

func (r *Repository) ApproveDeviceCode(ctx context.Context, userCode string, userID int64, at time.Time) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.ApproveDeviceCode", `
		UPDATE device_codes SET approved_at = ?, user_id = ? WHERE user_code = ? AND approved_at IS NULL
	`, at, userID, userCode)
	return err
}

func (r *Repository) ConsumeDeviceCode(ctx context.Context, deviceCode string, at time.Time) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "user.ConsumeDeviceCode", `
		UPDATE device_codes SET consumed_at = ? WHERE device_code = ? AND consumed_at IS NULL
	`, at, deviceCode)
	return err
}

func (r *Repository) FindActiveUserByID(ctx context.Context, userID int64) (shared.UserRecord, error) {
	var u shared.UserRecord
	err := dbutil.GetSQL(ctx, r.db, "user.FindActiveUserByID", &u, `
		SELECT id, email, name, tenant_id, active, created_at
		FROM users
		WHERE id = ? AND active = 1
		LIMIT 1
	`, userID)
	return u, err
}

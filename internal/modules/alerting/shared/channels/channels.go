// Package channels holds the notification-channel reads and delivery
// bookkeeping shared by the evaluator and the notifications API.
package channels

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

// Columns selects a models.ChannelRow from optikk.notification_channels.
const Columns = `id, tenant_id, type, name, config_json, status,
  last_used_at, last_delivery_at, last_error_text, created_at, updated_at`

// ByIDs loads the tenant's channels among ids.
func ByIDs(ctx context.Context, db *sqlx.DB, tenantID int64, ids []int64) ([]models.ChannelRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	q, args, err := sqlx.In(`SELECT `+Columns+` FROM optikk.notification_channels
		 WHERE tenant_id = ? AND id IN (?)`, tenantID, ids)
	if err != nil {
		return nil, err
	}
	var rows []models.ChannelRow
	err = dbutil.SelectSQL(ctx, db, "channels.ByIDs", &rows, db.Rebind(q), args...)
	return rows, err
}

// MarkDelivered records a delivery attempt: status ok on success, warn with
// the error text on failure.
func MarkDelivered(ctx context.Context, db *sqlx.DB, id int64, at time.Time, deliveryErr error) error {
	status, errText := "ok", sql.NullString{}
	if deliveryErr != nil {
		status, errText = "warn", sql.NullString{Valid: true, String: deliveryErr.Error()}
	}
	_, err := dbutil.ExecSQL(ctx, db, "channels.MarkDelivered", `
		UPDATE optikk.notification_channels
		   SET last_used_at = ?, last_delivery_at = ?, last_error_text = ?, status = ?
		 WHERE id = ?`, at, at, errText, status, id)
	return err
}

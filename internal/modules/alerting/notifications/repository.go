package notifications

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/alerting/shared/channels"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql")}
}

func (r *Repository) CreateChannel(ctx context.Context, row models.ChannelRow) (int64, error) {
	res, err := dbutil.ExecSQL(ctx, r.db, "notifications.CreateChannel", `
		INSERT INTO optikk.notification_channels
		  (tenant_id, type, name, config_json, status, created_at)
		VALUES (?, ?, ?, ?, 'ok', ?)
	`, row.TenantID, row.Type, row.Name, row.ConfigJSON, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repository) UpdateChannel(ctx context.Context, id, tenantID int64, row models.ChannelRow) error {
	return dbutil.ExecMatched(ctx, r.db, "notifications.UpdateChannel", `
		UPDATE optikk.notification_channels
		   SET type = ?, name = ?, config_json = ?, updated_at = ?
		 WHERE id = ? AND tenant_id = ?
	`, row.Type, row.Name, row.ConfigJSON, time.Now().UTC(), id, tenantID)
}

func (r *Repository) DeleteChannel(ctx context.Context, id, tenantID int64) error {
	return dbutil.ExecMatched(ctx, r.db, "notifications.DeleteChannel",
		`DELETE FROM optikk.notification_channels WHERE id = ? AND tenant_id = ?`, id, tenantID)
}

func (r *Repository) GetChannel(ctx context.Context, id, tenantID int64) (models.ChannelRow, error) {
	var row models.ChannelRow
	err := dbutil.GetSQL(ctx, r.db, "notifications.GetChannel", &row,
		`SELECT `+channels.Columns+` FROM optikk.notification_channels WHERE id = ? AND tenant_id = ? LIMIT 1`,
		id, tenantID)
	return row, err
}

func (r *Repository) ListChannels(ctx context.Context, tenantID int64) ([]models.ChannelRow, error) {
	var rows []models.ChannelRow
	err := dbutil.SelectSQL(ctx, r.db, "notifications.ListChannels", &rows,
		`SELECT `+channels.Columns+` FROM optikk.notification_channels WHERE tenant_id = ? ORDER BY created_at DESC`,
		tenantID)
	return rows, err
}

// CountChannelUsage returns how many of the tenant's monitors notify each
// channel, keyed by channel id.
func (r *Repository) CountChannelUsage(ctx context.Context, tenantID int64) (map[int64]int, error) {
	var rows []struct {
		ChannelID int64 `db:"channel_id"`
		Monitors  int   `db:"monitors"`
	}
	err := dbutil.SelectSQL(ctx, r.db, "notifications.CountChannelUsage", &rows, `
		SELECT j.channel_id AS channel_id, COUNT(*) AS monitors
		  FROM optikk.monitors m
		  JOIN JSON_TABLE(m.notify_json, '$.channelIds[*]'
		         COLUMNS (channel_id BIGINT PATH '$')) AS j
		 WHERE m.tenant_id = ?
		 GROUP BY j.channel_id`, tenantID)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]int, len(rows))
	for _, row := range rows {
		out[row.ChannelID] = row.Monitors
	}
	return out, nil
}

func (r *Repository) MarkChannelDelivered(ctx context.Context, id int64, at time.Time, deliveryErr error) error {
	return channels.MarkDelivered(ctx, r.db, id, at, deliveryErr)
}

const policyCols = `id, tenant_id, name, match_dsl, actions_json, hits_30d,
  last_used_at, enabled, position, created_at, updated_at`

func (r *Repository) CreatePolicy(ctx context.Context, row models.PolicyRow) (int64, error) {
	res, err := dbutil.ExecSQL(ctx, r.db, "notifications.CreatePolicy", `
		INSERT INTO optikk.notification_policies
		  (tenant_id, name, match_dsl, actions_json, enabled, position, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, row.TenantID, row.Name, row.MatchDSL, row.ActionsJSON, row.Enabled, row.Position, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repository) UpdatePolicy(ctx context.Context, id, tenantID int64, row models.PolicyRow) error {
	return dbutil.ExecMatched(ctx, r.db, "notifications.UpdatePolicy", `
		UPDATE optikk.notification_policies
		   SET name = ?, match_dsl = ?, actions_json = ?, enabled = ?, position = ?, updated_at = ?
		 WHERE id = ? AND tenant_id = ?
	`, row.Name, row.MatchDSL, row.ActionsJSON, row.Enabled, row.Position, time.Now().UTC(), id, tenantID)
}

func (r *Repository) DeletePolicy(ctx context.Context, id, tenantID int64) error {
	return dbutil.ExecMatched(ctx, r.db, "notifications.DeletePolicy",
		`DELETE FROM optikk.notification_policies WHERE id = ? AND tenant_id = ?`, id, tenantID)
}

func (r *Repository) GetPolicy(ctx context.Context, id, tenantID int64) (models.PolicyRow, error) {
	var row models.PolicyRow
	err := dbutil.GetSQL(ctx, r.db, "notifications.GetPolicy", &row,
		`SELECT `+policyCols+` FROM optikk.notification_policies WHERE id = ? AND tenant_id = ?`, id, tenantID)
	return row, err
}

func (r *Repository) ListPolicies(ctx context.Context, tenantID int64) ([]models.PolicyRow, error) {
	var rows []models.PolicyRow
	err := dbutil.SelectSQL(ctx, r.db, "notifications.ListPolicies", &rows,
		`SELECT `+policyCols+` FROM optikk.notification_policies WHERE tenant_id = ? ORDER BY position ASC, id ASC`,
		tenantID)
	return rows, err
}

const templateCols = `id, tenant_id, name, description, body, used_count, created_at, updated_at`

func (r *Repository) CreateTemplate(ctx context.Context, row models.TemplateRow) (int64, error) {
	res, err := dbutil.ExecSQL(ctx, r.db, "notifications.CreateTemplate", `
		INSERT INTO optikk.notification_templates
		  (tenant_id, name, description, body, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, row.TenantID, row.Name, row.Description, row.Body, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repository) UpdateTemplate(ctx context.Context, id, tenantID int64, row models.TemplateRow) error {
	return dbutil.ExecMatched(ctx, r.db, "notifications.UpdateTemplate", `
		UPDATE optikk.notification_templates
		   SET name = ?, description = ?, body = ?, updated_at = ?
		 WHERE id = ? AND tenant_id = ?
	`, row.Name, row.Description, row.Body, time.Now().UTC(), id, tenantID)
}

func (r *Repository) DeleteTemplate(ctx context.Context, id, tenantID int64) error {
	return dbutil.ExecMatched(ctx, r.db, "notifications.DeleteTemplate",
		`DELETE FROM optikk.notification_templates WHERE id = ? AND tenant_id = ?`, id, tenantID)
}

func (r *Repository) GetTemplate(ctx context.Context, id, tenantID int64) (models.TemplateRow, error) {
	var row models.TemplateRow
	err := dbutil.GetSQL(ctx, r.db, "notifications.GetTemplate", &row,
		`SELECT `+templateCols+` FROM optikk.notification_templates WHERE id = ? AND tenant_id = ?`, id, tenantID)
	return row, err
}

func (r *Repository) ListTemplates(ctx context.Context, tenantID int64) ([]models.TemplateRow, error) {
	var rows []models.TemplateRow
	err := dbutil.SelectSQL(ctx, r.db, "notifications.ListTemplates", &rows,
		`SELECT `+templateCols+` FROM optikk.notification_templates WHERE tenant_id = ? ORDER BY created_at DESC`,
		tenantID)
	return rows, err
}

func (r *Repository) ChannelInUse(ctx context.Context, channelID, tenantID int64) (bool, error) {
	var cnt int
	err := dbutil.GetSQL(ctx, r.db, "notifications.ChannelInUse", &cnt, `
		SELECT COUNT(*)
		  FROM optikk.monitors m
		  JOIN JSON_TABLE(m.notify_json, '$.channelIds[*]'
		         COLUMNS (channel_id BIGINT PATH '$')) AS j
		 WHERE m.tenant_id = ? AND j.channel_id = ?
	`, tenantID, channelID)
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

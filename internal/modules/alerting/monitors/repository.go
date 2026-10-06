package monitors

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql")}
}

// Create inserts the monitor and its initial no_data state in one
// transaction and returns the new monitor id.
func (r *Repository) Create(ctx context.Context, m models.MonitorRow) (int64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO optikk.monitors
		  (tenant_id, name, type, priority, scope_json, query_json, conditions_json, notify_json,
		   message_body, runbook_url, tags_json, eval_every_sec, renotify_every_sec,
		   active, created_at, created_by_user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		m.TenantID, m.Name, m.Type, m.Priority, m.Scope, m.Query, m.Conditions, m.Notify,
		m.MessageBody, m.RunbookURL, m.Tags, m.EvalEverySec, m.RenotifyEverySec,
		now, m.CreatedByUserID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO optikk.monitor_state (monitor_id, status, next_evaluation_at)
		VALUES (?, 'no_data', ?)`, id, now); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (r *Repository) Update(ctx context.Context, id int64, m models.MonitorRow) error {
	return dbutil.ExecMatched(ctx, r.db, "monitors.Update", `
		UPDATE optikk.monitors
		   SET name = ?, type = ?, priority = ?,
		       scope_json = ?, query_json = ?, conditions_json = ?, notify_json = ?,
		       message_body = ?, runbook_url = ?, tags_json = ?,
		       eval_every_sec = ?, renotify_every_sec = ?, updated_at = ?
		 WHERE id = ? AND tenant_id = ?`,
		m.Name, m.Type, m.Priority, m.Scope, m.Query, m.Conditions, m.Notify,
		m.MessageBody, m.RunbookURL, m.Tags, m.EvalEverySec, m.RenotifyEverySec,
		time.Now().UTC(), id, m.TenantID)
}

// Delete removes the monitor with its state and events in one transaction.
func (r *Repository) Delete(ctx context.Context, id, tenantID int64) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`DELETE FROM optikk.monitors WHERE id = ? AND tenant_id = ?`, id, tenantID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return sql.ErrNoRows
	}
	for _, table := range []string{"optikk.monitor_state", "optikk.monitor_events"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE monitor_id = ?`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const monitorWithStateFrom = `
	  FROM optikk.monitors m
	  JOIN optikk.monitor_state s ON s.monitor_id = m.id`

func (r *Repository) GetByID(ctx context.Context, id, tenantID int64) (models.MonitorRow, models.MonitorStateRow, error) {
	var row models.MonitorWithStateRow
	err := dbutil.GetSQL(ctx, r.db, "monitors.GetByID", &row,
		`SELECT `+models.MonitorWithStateColumns+monitorWithStateFrom+`
		 WHERE m.id = ? AND m.tenant_id = ?
		 LIMIT 1`, id, tenantID)
	if err != nil {
		return models.MonitorRow{}, models.MonitorStateRow{}, err
	}
	return row.MonitorRow, row.MonitorStateRow, nil
}

func monitorListWhere(tenantID int64, q ListQuery) (string, []any) {
	where := []string{"m.tenant_id = ?"}
	args := []any{tenantID}
	if q.Type != "" {
		where = append(where, "m.type = ?")
		args = append(args, q.Type)
	}
	if q.Priority != "" {
		where = append(where, "m.priority = ?")
		args = append(args, q.Priority)
	}
	if len(q.Statuses) > 0 {
		where = append(where, "s.status IN (?"+strings.Repeat(", ?", len(q.Statuses)-1)+")")
		for _, s := range q.Statuses {
			args = append(args, s)
		}
	}
	if q.Muted != nil {
		if *q.Muted {
			where = append(where, "m.muted_until IS NOT NULL AND m.muted_until > NOW()")
		} else {
			where = append(where, "(m.muted_until IS NULL OR m.muted_until <= NOW())")
		}
	}
	if q.Search != "" {
		where = append(where, "m.name LIKE ?")
		args = append(args, filterutil.LikeSubstringPattern(q.Search))
	}
	return strings.Join(where, " AND "), args
}

func (r *Repository) List(ctx context.Context, tenantID int64, q ListQuery) ([]models.MonitorWithStateRow, error) {
	where, args := monitorListWhere(tenantID, q)
	args = append(args, q.Limit, q.Offset)

	var rows []models.MonitorWithStateRow
	err := dbutil.SelectSQL(ctx, r.db, "monitors.List", &rows,
		`SELECT `+models.MonitorWithStateColumns+monitorWithStateFrom+`
		 WHERE `+where+`
		 ORDER BY m.created_at DESC, m.id DESC
		 LIMIT ? OFFSET ?`, args...)
	return rows, err
}

func (r *Repository) Count(ctx context.Context, tenantID int64, q ListQuery) (StatusCounts, error) {
	where, args := monitorListWhere(tenantID, q)
	var counts StatusCounts
	err := dbutil.GetSQL(ctx, r.db, "monitors.Count", &counts, `
		SELECT COUNT(*) AS total,
		       COALESCE(SUM(s.status = 'alert'), 0) AS alert,
		       COALESCE(SUM(s.status = 'warn'), 0) AS warn,
		       COALESCE(SUM(s.status = 'ok'), 0) AS ok,
		       COALESCE(SUM(s.status = 'no_data'), 0) AS no_data,
		       COALESCE(SUM(m.muted_until IS NOT NULL AND m.muted_until > NOW()), 0) AS muted`+
		monitorWithStateFrom+`
		 WHERE `+where, args...)
	return counts, err
}

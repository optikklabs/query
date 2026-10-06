package monitors

import (
	"context"
	"database/sql"
	"errors"
	"time"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

type EventRow struct {
	models.MonitorEventRow
	MonitorName string `db:"monitor_name"`
}

const eventWithMonitorSelect = `
		SELECT e.id, e.monitor_id, e.tenant_id, e.kind, e.value, e.threshold,
		       e.started_at, e.ended_at, e.resolved_by, e.peak_value, e.note,
		       m.name AS monitor_name
		  FROM optikk.monitor_events e
		  JOIN optikk.monitors m ON m.id = e.monitor_id`

func (r *Repository) Events(ctx context.Context, monitorID, tenantID int64, limit int) ([]EventRow, error) {
	var rows []EventRow
	err := dbutil.SelectSQL(ctx, r.db, "monitors.Events", &rows, eventWithMonitorSelect+`
		 WHERE e.monitor_id = ? AND e.tenant_id = ?
		 ORDER BY e.started_at DESC, e.id DESC
		 LIMIT ?`, monitorID, tenantID, limit)
	return rows, err
}

func (r *Repository) Activity(ctx context.Context, tenantID int64, since time.Time, limit int) ([]EventRow, error) {
	var rows []EventRow
	err := dbutil.SelectSQL(ctx, r.db, "monitors.Activity", &rows, eventWithMonitorSelect+`
		 WHERE e.tenant_id = ? AND e.started_at >= ?
		 ORDER BY e.started_at DESC, e.id DESC
		 LIMIT ?`, tenantID, since, limit)
	return rows, err
}

func (r *Repository) StatusTimelineRows(ctx context.Context, monitorID, tenantID int64, since time.Time) ([]models.MonitorEventRow, error) {
	var rows []models.MonitorEventRow
	const q = `
		SELECT id, monitor_id, tenant_id, kind, value, threshold, started_at,
		       ended_at, resolved_by, peak_value, note
		  FROM optikk.monitor_events
		 WHERE monitor_id = ? AND tenant_id = ? AND started_at >= ?
		 ORDER BY started_at ASC, id ASC
	`
	err := dbutil.SelectSQL(ctx, r.db, "monitors.StatusTimelineRows", &rows, q, monitorID, tenantID, since)
	return rows, err
}

// LastEventKindBefore returns the kind of the monitor's latest event before
// t, or "" when it has none.
func (r *Repository) LastEventKindBefore(ctx context.Context, monitorID, tenantID int64, t time.Time) (string, error) {
	var kind string
	err := dbutil.GetSQL(ctx, r.db, "monitors.LastEventKindBefore", &kind, `
		SELECT kind FROM optikk.monitor_events
		 WHERE monitor_id = ? AND tenant_id = ? AND started_at < ?
		 ORDER BY started_at DESC, id DESC
		 LIMIT 1`, monitorID, tenantID, t)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return kind, err
}

func (r *Repository) Ack(ctx context.Context, monitorID, tenantID, userID int64, at time.Time) error {
	const q = `
		UPDATE optikk.monitor_state s
		   JOIN optikk.monitors m ON m.id = s.monitor_id
		    SET s.acked_by_user_id = ?, s.acked_at = ?
		  WHERE s.monitor_id = ? AND m.tenant_id = ? AND s.status IN ('alert','warn')
	`
	return dbutil.ExecMatched(ctx, r.db, "monitors.Ack", q, userID, at, monitorID, tenantID)
}

func (r *Repository) Mute(ctx context.Context, monitorID, tenantID int64, until sql.NullTime) error {
	const q = `UPDATE optikk.monitors SET muted_until = ?, updated_at = ? WHERE id = ? AND tenant_id = ?`
	return dbutil.ExecMatched(ctx, r.db, "monitors.Mute", q, until, time.Now().UTC(), monitorID, tenantID)
}

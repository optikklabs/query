// Package evaluator runs the BackgroundRunner that ticks the alerting platform.
package evaluator

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql")}
}

type DueMonitor struct {
	Monitor models.MonitorRow
	State   models.MonitorStateRow
}

type UpdateStateArgs struct {
	MonitorID          int64
	PrevStatus         string
	NewStatus          string
	CurrentValue       sql.NullFloat64
	LastEvaluatedAt    time.Time
	NextEvaluationAt   time.Time
	TriggeredAt        sql.NullTime
	LastNotifiedAt     sql.NullTime
	IncrementEvalCount bool
}

// claimLease is how long a claimed monitor stays invisible to other
// replicas. UpdateState releases the claim early on completion; the lease
// only matters when a replica crashes mid-evaluation.
const claimLease = 5 * time.Minute

// ClaimDue atomically claims up to limit due monitors for one evaluator
// replica and returns them. MySQL is the only coordinator: the claim UPDATE
// is atomic, expired leases are reclaimable, so any number of replicas can
// run the tick loop concurrently without duplicate evaluations.
func (r *Repository) ClaimDue(ctx context.Context, claimID string, now time.Time, limit int) ([]DueMonitor, error) {
	// Single-table UPDATE: MySQL forbids ORDER BY/LIMIT on multi-table
	// updates, so active-monitor filtering uses a subquery instead of a JOIN.
	const claim = `
		UPDATE optikk.monitor_state
		   SET claimed_by = ?, claimed_until = ?
		 WHERE next_evaluation_at <= ?
		   AND (claimed_until IS NULL OR claimed_until < ?)
		   AND monitor_id IN (SELECT id FROM optikk.monitors WHERE active = 1)
		 ORDER BY next_evaluation_at
		 LIMIT ?
	`
	res, err := dbutil.ExecSQL(ctx, r.db, "evaluator.ClaimDue", claim,
		claimID, now.Add(claimLease), now, now, limit)
	if err != nil {
		return nil, err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return nil, nil
	}

	const query = `
		SELECT
		  m.id, m.tenant_id, m.name, m.type, m.priority,
		  m.scope_json, m.query_json, m.conditions_json, m.notify_json,
		  m.message_template_id, m.message_body, m.runbook_url, m.tags_json,
		  m.eval_every_sec, m.renotify_every_sec, m.muted_until, m.active,
		  m.created_at, m.updated_at, m.created_by_user_id,
		  s.monitor_id        AS s_monitor_id,
		  s.status            AS s_status,
		  s.current_value     AS s_current_value,
		  s.last_evaluated_at AS s_last_evaluated_at,
		  s.next_evaluation_at AS s_next_evaluation_at,
		  s.triggered_at      AS s_triggered_at,
		  s.last_notified_at  AS s_last_notified_at,
		  s.evaluation_count  AS s_evaluation_count,
		  s.acked_by_user_id  AS s_acked_by_user_id,
		  s.acked_at          AS s_acked_at
		FROM optikk.monitors m
		JOIN optikk.monitor_state s ON s.monitor_id = m.id
		WHERE s.claimed_by = ?
		ORDER BY s.next_evaluation_at
	`
	var raw []dueRow
	if err := dbutil.SelectSQL(ctx, r.db, "evaluator.LoadClaimed", &raw, query, claimID); err != nil {
		return nil, err
	}
	out := make([]DueMonitor, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.toDue())
	}
	return out, nil
}

type dueRow struct {
	models.MonitorRow
	SMonitorID        sql.NullInt64   `db:"s_monitor_id"`
	SStatus           sql.NullString  `db:"s_status"`
	SCurrentValue     sql.NullFloat64 `db:"s_current_value"`
	SLastEvaluatedAt  sql.NullTime    `db:"s_last_evaluated_at"`
	SNextEvaluationAt sql.NullTime    `db:"s_next_evaluation_at"`
	STriggeredAt      sql.NullTime    `db:"s_triggered_at"`
	SLastNotifiedAt   sql.NullTime    `db:"s_last_notified_at"`
	SEvaluationCount  sql.NullInt64   `db:"s_evaluation_count"`
	SAckedByUserID    sql.NullInt64   `db:"s_acked_by_user_id"`
	SAckedAt          sql.NullTime    `db:"s_acked_at"`
}

func (r dueRow) toDue() DueMonitor {
	state := models.MonitorStateRow{
		MonitorID:       r.SMonitorID.Int64,
		Status:          r.SStatus.String,
		CurrentValue:    r.SCurrentValue,
		LastEvaluatedAt: r.SLastEvaluatedAt,
		TriggeredAt:     r.STriggeredAt,
		LastNotifiedAt:  r.SLastNotifiedAt,
		EvaluationCount: r.SEvaluationCount.Int64,
		AckedByUserID:   r.SAckedByUserID,
		AckedAt:         r.SAckedAt,
	}
	if r.SNextEvaluationAt.Valid {
		state.NextEvaluationAt = r.SNextEvaluationAt.Time
	}
	return DueMonitor{Monitor: r.MonitorRow, State: state}
}

func (r *Repository) UpdateState(ctx context.Context, args UpdateStateArgs) error {

	// Clearing claimed_by/claimed_until releases the work claim taken by
	// ClaimDue as soon as this monitor's evaluation completes.
	q := `
		UPDATE optikk.monitor_state
		   SET status = ?, current_value = ?, last_evaluated_at = ?, next_evaluation_at = ?,
		       triggered_at = ?, last_notified_at = COALESCE(?, last_notified_at),
		       evaluation_count = evaluation_count + ?,
		       claimed_by = NULL, claimed_until = NULL
		 WHERE monitor_id = ? AND status = ?
	`
	incr := 0
	if args.IncrementEvalCount {
		incr = 1
	}
	_, err := dbutil.ExecSQL(ctx, r.db, "evaluator.UpdateState", q,
		args.NewStatus, args.CurrentValue, args.LastEvaluatedAt, args.NextEvaluationAt,
		args.TriggeredAt, args.LastNotifiedAt, incr,
		args.MonitorID, args.PrevStatus)
	return err
}

func (r *Repository) InsertEvent(ctx context.Context, e models.MonitorEventRow) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "evaluator.InsertEvent", `
		INSERT INTO optikk.monitor_events
		  (monitor_id, tenant_id, kind, value, threshold, started_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, e.MonitorID, e.TenantID, e.Kind, e.Value, e.Threshold, e.StartedAt)
	return err
}

func (r *Repository) GetChannelsByIDs(ctx context.Context, tenantID int64, ids []int64) ([]models.ChannelRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	q, args, err := sqlx.In(`
		SELECT id, tenant_id, type, name, config_json, status,
		       last_used_at, last_delivery_at, last_error_text, created_at, updated_at
		  FROM optikk.notification_channels
		 WHERE tenant_id = ? AND id IN (?)
	`, tenantID, ids)
	if err != nil {
		return nil, err
	}
	q = r.db.Rebind(q)
	var rows []models.ChannelRow
	if err := dbutil.SelectSQL(ctx, r.db, "evaluator.GetChannelsByIDs", &rows, q, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) MarkChannelDelivered(ctx context.Context, id int64, at time.Time, errText sql.NullString) error {
	status := "ok"
	if errText.Valid && errText.String != "" {
		status = "warn"
	}
	_, err := dbutil.ExecSQL(ctx, r.db, "evaluator.MarkChannelDelivered", `
		UPDATE optikk.notification_channels
		   SET last_used_at = ?, last_delivery_at = ?, last_error_text = ?, status = ?
		 WHERE id = ?
	`, at, at, errText, status, id)
	return err
}

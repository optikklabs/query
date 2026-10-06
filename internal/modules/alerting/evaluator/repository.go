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
	NoDataSince        sql.NullTime
	IncrementEvalCount bool
}

const claimLease = 5 * time.Minute

func (r *Repository) ClaimDue(ctx context.Context, claimID string, now time.Time, limit int) ([]DueMonitor, error) {
	const claim = `
		UPDATE optikk.monitor_state
		   SET claimed_by = ?, claimed_until = ?
		 WHERE next_evaluation_at <= ?
		   AND (claimed_until IS NULL OR claimed_until < ?)
		   AND monitor_id IN (SELECT id FROM optikk.monitors WHERE active = 1)
		 ORDER BY next_evaluation_at, monitor_id
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

	query := `SELECT ` + models.MonitorWithStateColumns + `
		FROM optikk.monitors m
		JOIN optikk.monitor_state s ON s.monitor_id = m.id
		WHERE s.claimed_by = ?
		ORDER BY s.next_evaluation_at, s.monitor_id`
	var rows []models.MonitorWithStateRow
	if err := dbutil.SelectSQL(ctx, r.db, "evaluator.LoadClaimed", &rows, query, claimID); err != nil {
		return nil, err
	}
	out := make([]DueMonitor, 0, len(rows))
	for _, row := range rows {
		m, state := row.Split()
		out = append(out, DueMonitor{Monitor: m, State: state})
	}
	return out, nil
}

func (r *Repository) UpdateState(ctx context.Context, args UpdateStateArgs) error {
	const q = `
		UPDATE optikk.monitor_state
		   SET status = ?, current_value = ?, last_evaluated_at = ?, next_evaluation_at = ?,
		       triggered_at = ?, last_notified_at = COALESCE(?, last_notified_at),
		       no_data_since = ?, evaluation_count = evaluation_count + ?,
		       claimed_by = NULL, claimed_until = NULL
		 WHERE monitor_id = ? AND status = ?
	`
	incr := 0
	if args.IncrementEvalCount {
		incr = 1
	}
	_, err := dbutil.ExecSQL(ctx, r.db, "evaluator.UpdateState", q,
		args.NewStatus, args.CurrentValue, args.LastEvaluatedAt, args.NextEvaluationAt,
		args.TriggeredAt, args.LastNotifiedAt, args.NoDataSince, incr,
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

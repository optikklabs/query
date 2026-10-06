package evaluator

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
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return nil, err
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
		out = append(out, DueMonitor{Monitor: row.MonitorRow, State: row.MonitorStateRow})
	}
	return out, nil
}

// UpdateState applies args only while the stored status is still
// args.PrevStatus, returning sql.ErrNoRows when another evaluation won.
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
	return dbutil.ExecMatched(ctx, r.db, "evaluator.UpdateState", q,
		args.NewStatus, args.CurrentValue, args.LastEvaluatedAt, args.NextEvaluationAt,
		args.TriggeredAt, args.LastNotifiedAt, args.NoDataSince, incr,
		args.MonitorID, args.PrevStatus)
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
	return channels.ByIDs(ctx, r.db, tenantID, ids)
}

func (r *Repository) MarkChannelDelivered(ctx context.Context, id int64, at time.Time, deliveryErr error) error {
	return channels.MarkDelivered(ctx, r.db, id, at, deliveryErr)
}

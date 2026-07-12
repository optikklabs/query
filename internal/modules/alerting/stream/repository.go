package stream

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

type Repository struct{ db *sqlx.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: sqlx.NewDb(db, "mysql")} }

// LoadActive is a one-time bootstrap read. The evaluator never polls this
// table; live config updates will be supplied by the config Kafka topic.
func (r *Repository) LoadActive(ctx context.Context) ([]models.MonitorRow, []models.MonitorStateRow, error) {
	const q = `
		SELECT m.id, m.tenant_id, m.name, m.type, m.priority,
		       m.scope_json, m.query_json, m.conditions_json, m.notify_json,
		       m.message_template_id, m.message_body, m.runbook_url, m.tags_json,
		       m.eval_every_sec, m.renotify_every_sec, m.muted_until, m.active,
		       m.created_at, m.updated_at, m.created_by_user_id,
		       s.monitor_id AS state_monitor_id, s.status AS state_status,
		       s.current_value AS state_current_value, s.last_evaluated_at AS state_last_evaluated_at,
		       s.next_evaluation_at AS state_next_evaluation_at, s.triggered_at AS state_triggered_at,
		       s.last_notified_at AS state_last_notified_at, s.evaluation_count AS state_evaluation_count,
		       s.acked_by_user_id AS state_acked_by_user_id, s.acked_at AS state_acked_at
		  FROM optikk.monitors m JOIN optikk.monitor_state s ON s.monitor_id = m.id
		 WHERE m.active = 1`
	var rows []bootstrapRow
	if err := dbutil.SelectSQL(ctx, r.db, "alerting.stream.LoadActive", &rows, q); err != nil {
		return nil, nil, err
	}
	monitors := make([]models.MonitorRow, 0, len(rows))
	states := make([]models.MonitorStateRow, 0, len(rows))
	for _, row := range rows {
		monitors = append(monitors, row.MonitorRow)
		states = append(states, row.state())
	}
	return monitors, states, nil
}

type bootstrapRow struct {
	models.MonitorRow
	StateMonitorID        sql.NullInt64   `db:"state_monitor_id"`
	StateStatus           sql.NullString  `db:"state_status"`
	StateCurrentValue     sql.NullFloat64 `db:"state_current_value"`
	StateLastEvaluatedAt  sql.NullTime    `db:"state_last_evaluated_at"`
	StateNextEvaluationAt sql.NullTime    `db:"state_next_evaluation_at"`
	StateTriggeredAt      sql.NullTime    `db:"state_triggered_at"`
	StateLastNotifiedAt   sql.NullTime    `db:"state_last_notified_at"`
	StateEvaluationCount  sql.NullInt64   `db:"state_evaluation_count"`
	StateAckedByUserID    sql.NullInt64   `db:"state_acked_by_user_id"`
	StateAckedAt          sql.NullTime    `db:"state_acked_at"`
}

func (r bootstrapRow) state() models.MonitorStateRow {
	return models.MonitorStateRow{MonitorID: r.StateMonitorID.Int64, Status: r.StateStatus.String, CurrentValue: r.StateCurrentValue, LastEvaluatedAt: r.StateLastEvaluatedAt, NextEvaluationAt: r.StateNextEvaluationAt.Time, TriggeredAt: r.StateTriggeredAt, LastNotifiedAt: r.StateLastNotifiedAt, EvaluationCount: r.StateEvaluationCount.Int64, AckedByUserID: r.StateAckedByUserID, AckedAt: r.StateAckedAt}
}

// PersistTransition projects a state transition to MySQL for the API/audit
// read model. It is deliberately called only for transitions or renotifies,
// never once per metric record.
func (r *Repository) PersistTransition(ctx context.Context, t Transition) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
		UPDATE optikk.monitor_state
		   SET status = ?, current_value = ?, last_evaluated_at = ?, triggered_at = ?,
		       last_notified_at = ?, evaluation_count = evaluation_count + 1
		 WHERE monitor_id = ?`, t.State.Status, t.State.CurrentValue, t.State.LastEvaluatedAt,
		t.State.TriggeredAt, t.State.LastNotifiedAt, t.Monitor.ID)
	if err != nil {
		return fmt.Errorf("update monitor state: %w", err)
	}
	if t.Decision.Transition {
		kind := "triggered"
		if t.Decision.IsRecovery {
			kind = "recovered"
		}
		threshold := threshold(t.Monitor.Conditions)
		if _, err := tx.ExecContext(ctx, `INSERT INTO optikk.monitor_events (monitor_id, tenant_id, kind, value, threshold, started_at) VALUES (?, ?, ?, ?, ?, ?)`, t.Monitor.ID, t.Monitor.TenantID, kind, t.Value, threshold, t.At); err != nil {
			return fmt.Errorf("insert monitor event: %w", err)
		}
	}
	return tx.Commit()
}

func threshold(c models.Conditions) any {
	if c.AlertThreshold != nil {
		return *c.AlertThreshold
	}
	if c.WarnThreshold != nil {
		return *c.WarnThreshold
	}
	return nil
}

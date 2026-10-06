package models

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/optikklabs/query/internal/shared/sqljson"
)

// Monitor statuses, as stored in monitor_state.status.
const (
	StatusAlert  = "alert"
	StatusWarn   = "warn"
	StatusOK     = "ok"
	StatusNoData = "no_data"
)

// IsFiring reports whether status is one that notifies and acks.
func IsFiring(status string) bool {
	return status == StatusAlert || status == StatusWarn
}

// Monitor event kinds, as stored in monitor_events.kind.
const (
	EventTriggered = "triggered"
	EventRecovered = "recovered"
)

// Comparators for Conditions.Comparator.
const (
	ComparatorAbove = "above"
	ComparatorBelow = "below"
	ComparatorEqual = "equal"
)

type MonitorRow struct {
	ID                int64              `db:"id"`
	TenantID          int64              `db:"tenant_id"`
	Name              string             `db:"name"`
	Type              string             `db:"type"`
	Priority          string             `db:"priority"`
	Scope             Scope              `db:"scope_json"`
	Query             MonitorQuery       `db:"query_json"`
	Conditions        Conditions         `db:"conditions_json"`
	Notify            NotifyTargets      `db:"notify_json"`
	MessageTemplateID sql.NullInt64      `db:"message_template_id"`
	MessageBody       sql.NullString     `db:"message_body"`
	RunbookURL        sql.NullString     `db:"runbook_url"`
	Tags              sqljson.StringList `db:"tags_json"`
	EvalEverySec      int                `db:"eval_every_sec"`
	RenotifyEverySec  sql.NullInt64      `db:"renotify_every_sec"`
	MutedUntil        sql.NullTime       `db:"muted_until"`
	Active            bool               `db:"active"`
	CreatedAt         time.Time          `db:"created_at"`
	UpdatedAt         sql.NullTime       `db:"updated_at"`
	CreatedByUserID   sql.NullInt64      `db:"created_by_user_id"`
}

type MonitorStateRow struct {
	MonitorID        int64           `db:"monitor_id"`
	Status           string          `db:"status"`
	CurrentValue     sql.NullFloat64 `db:"current_value"`
	LastEvaluatedAt  sql.NullTime    `db:"last_evaluated_at"`
	NextEvaluationAt time.Time       `db:"next_evaluation_at"`
	TriggeredAt      sql.NullTime    `db:"triggered_at"`
	LastNotifiedAt   sql.NullTime    `db:"last_notified_at"`
	EvaluationCount  int64           `db:"evaluation_count"`
	AckedByUserID    sql.NullInt64   `db:"acked_by_user_id"`
	AckedAt          sql.NullTime    `db:"acked_at"`
	NoDataSince      sql.NullTime    `db:"no_data_since"`
}

// MonitorWithStateColumns selects a monitor (alias m) joined with its state
// (alias s) into a MonitorWithStateRow.
const MonitorWithStateColumns = `
  m.id, m.tenant_id, m.name, m.type, m.priority,
  m.scope_json, m.query_json, m.conditions_json, m.notify_json,
  m.message_template_id, m.message_body, m.runbook_url, m.tags_json,
  m.eval_every_sec, m.renotify_every_sec, m.muted_until, m.active,
  m.created_at, m.updated_at, m.created_by_user_id,
  s.monitor_id, s.status, s.current_value, s.last_evaluated_at,
  s.next_evaluation_at, s.triggered_at, s.last_notified_at,
  s.evaluation_count, s.acked_by_user_id, s.acked_at, s.no_data_since
`

// MonitorWithStateRow scans MonitorWithStateColumns. Every monitor has a
// state row, created with it.
type MonitorWithStateRow struct {
	MonitorRow
	MonitorStateRow
}

type MonitorEventRow struct {
	ID         int64           `db:"id"`
	MonitorID  int64           `db:"monitor_id"`
	TenantID   int64           `db:"tenant_id"`
	Kind       string          `db:"kind"`
	Value      sql.NullFloat64 `db:"value"`
	Threshold  sql.NullFloat64 `db:"threshold"`
	StartedAt  time.Time       `db:"started_at"`
	EndedAt    sql.NullTime    `db:"ended_at"`
	ResolvedBy sql.NullString  `db:"resolved_by"`
	PeakValue  sql.NullFloat64 `db:"peak_value"`
	Note       sql.NullString  `db:"note"`
}

type Scope struct {
	Tags []ScopeTag `json:"tags,omitempty"`
}

func (s *Scope) Scan(src any) error { return sqljson.Scan(src, s) }

func (s Scope) Value() (driver.Value, error) { return json.Marshal(s) }

type ScopeTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Conditions are validated on write: Comparator, AlertThreshold and NoDataAs
// are always set.
type Conditions struct {
	Comparator        string   `json:"comparator"`
	AlertThreshold    *float64 `json:"alertThreshold,omitempty"`
	WarnThreshold     *float64 `json:"warnThreshold,omitempty"`
	RecoveryThreshold *float64 `json:"recoveryThreshold,omitempty"`
	NoDataAfterSec    int      `json:"noDataAfterSec"`
	// NoDataAs is the status a data gap resolves to: no_data, alert or ok.
	NoDataAs  string `json:"noDataAs"`
	MinSample *int   `json:"minSample,omitempty"`
}

func (c *Conditions) Scan(src any) error { return sqljson.Scan(src, c) }

func (c Conditions) Value() (driver.Value, error) { return json.Marshal(c) }

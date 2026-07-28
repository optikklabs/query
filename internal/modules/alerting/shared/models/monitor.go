package models

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type MonitorRow struct {
	ID                int64          `db:"id"`
	TenantID          int64          `db:"tenant_id"`
	Name              string         `db:"name"`
	Type              string         `db:"type"`
	Priority          string         `db:"priority"`
	Scope             Scope          `db:"scope_json"`
	Query             MonitorQuery   `db:"query_json"`
	Conditions        Conditions     `db:"conditions_json"`
	Notify            NotifyTargets  `db:"notify_json"`
	MessageTemplateID sql.NullInt64  `db:"message_template_id"`
	MessageBody       sql.NullString `db:"message_body"`
	RunbookURL        sql.NullString `db:"runbook_url"`
	Tags              Tags           `db:"tags_json"`
	EvalEverySec      int            `db:"eval_every_sec"`
	RenotifyEverySec  sql.NullInt64  `db:"renotify_every_sec"`
	MutedUntil        sql.NullTime   `db:"muted_until"`
	Active            bool           `db:"active"`
	CreatedAt         time.Time      `db:"created_at"`
	UpdatedAt         sql.NullTime   `db:"updated_at"`
	CreatedByUserID   sql.NullInt64  `db:"created_by_user_id"`
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

func (s *Scope) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &s)
}

func (s Scope) Value() (driver.Value, error) {
	return json.Marshal(s)
}

type ScopeTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Conditions struct {
	Comparator        string   `json:"comparator"`
	AlertThreshold    *float64 `json:"alertThreshold,omitempty"`
	WarnThreshold     *float64 `json:"warnThreshold,omitempty"`
	RecoveryThreshold *float64 `json:"recoveryThreshold,omitempty"`
	NoDataAfterSec    int      `json:"noDataAfterSec"`

	NoDataAs  string `json:"noDataAs"`
	MinSample *int   `json:"minSample,omitempty"`
}

func (c *Conditions) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &c)
}

func (c Conditions) Value() (driver.Value, error) {
	return json.Marshal(c)
}

type Tags []string

func (t *Tags) Scan(value interface{}) error {
	if value == nil {
		*t = nil
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &t)
}

func (t Tags) Value() (driver.Value, error) {
	if t == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(t)
}

package evaluator

import (
	"database/sql"
	"testing"
	"time"

	"github.com/optikklabs/query/internal/modules/alerting/shared/expr"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/modules/alerting/shared/query"
)

var now = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func TestRescheduleOnlyKeepsState(t *testing.T) {
	triggered := sql.NullTime{Valid: true, Time: now.Add(-time.Hour)}
	noData := sql.NullTime{Valid: true, Time: now.Add(-time.Minute)}
	state := models.MonitorStateRow{
		Status:       "alert",
		CurrentValue: sql.NullFloat64{Valid: true, Float64: 42},
		TriggeredAt:  triggered,
		NoDataSince:  noData,
	}
	got := rescheduleOnly(models.MonitorRow{ID: 9, EvalEverySec: 60}, state, now)

	if got.PrevStatus != "alert" || got.NewStatus != "alert" {
		t.Fatalf("status %s->%s, want alert->alert", got.PrevStatus, got.NewStatus)
	}
	if got.CurrentValue != state.CurrentValue || got.TriggeredAt != triggered || got.NoDataSince != noData {
		t.Fatalf("state not preserved: %+v", got)
	}
	if !got.NextEvaluationAt.Equal(now.Add(time.Minute)) || got.LastNotifiedAt.Valid {
		t.Fatalf("schedule = %+v", got)
	}
}

func TestBuildUpdateArgsTriggeredAt(t *testing.T) {
	m := models.MonitorRow{ID: 1, EvalEverySec: 30}
	res := query.ScalarResult{Value: 5, HasData: true}
	earlier := sql.NullTime{Valid: true, Time: now.Add(-time.Hour)}

	fresh := buildUpdateArgs(m, models.MonitorStateRow{Status: "ok"}, expr.Decision{NewStatus: "alert"}, res, now)
	if !fresh.TriggeredAt.Valid || !fresh.TriggeredAt.Time.Equal(now) {
		t.Fatalf("new alert TriggeredAt = %+v, want now", fresh.TriggeredAt)
	}

	ongoing := buildUpdateArgs(m, models.MonitorStateRow{Status: "alert", TriggeredAt: earlier}, expr.Decision{NewStatus: "alert"}, res, now)
	if ongoing.TriggeredAt != earlier {
		t.Fatalf("ongoing alert TriggeredAt = %+v, want %+v", ongoing.TriggeredAt, earlier)
	}

	recovered := buildUpdateArgs(m, models.MonitorStateRow{Status: "alert", TriggeredAt: earlier}, expr.Decision{NewStatus: "ok", ShouldNotify: true}, res, now)
	if recovered.TriggeredAt.Valid || !recovered.LastNotifiedAt.Valid {
		t.Fatalf("recovered args = %+v", recovered)
	}
}

func TestBuildPayload(t *testing.T) {
	alert := 10.0
	m := models.MonitorRow{
		Name:       "High errors",
		Priority:   "P1",
		Scope:      models.Scope{Tags: []models.ScopeTag{{Key: "service", Value: "api"}, {Key: "env", Value: "prod"}}},
		Conditions: models.Conditions{AlertThreshold: &alert},
	}
	p := buildPayload(m, "ok", query.ScalarResult{Value: 12.5, HasData: true}, expr.Decision{NewStatus: "alert"})
	if p.Transition != "ok->alert" || !p.IsAlert || p.IsRecovery || p.Threshold != 10 {
		t.Fatalf("payload = %+v", p)
	}
	if p.ScopeSummary != "service:api env:prod" {
		t.Fatalf("scope = %q", p.ScopeSummary)
	}
	if p.Message == "" {
		t.Fatal("empty default message")
	}
}

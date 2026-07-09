package evaluator

import (
	"database/sql"
	"testing"
	"time"

	"github.com/optikklabs/query/internal/modules/alerting/shared/expr"
	"github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/modules/alerting/shared/query"
)

func ptr(f float64) *float64 { return &f }

func TestIsMuted(t *testing.T) {
	now := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	if isMuted(models.MonitorRow{}, now) {
		t.Error("no MutedUntil -> not muted")
	}
	past := models.MonitorRow{MutedUntil: sql.NullTime{Valid: true, Time: now.Add(-time.Hour)}}
	if isMuted(past, now) {
		t.Error("past mute -> not muted")
	}
	future := models.MonitorRow{MutedUntil: sql.NullTime{Valid: true, Time: now.Add(time.Hour)}}
	if !isMuted(future, now) {
		t.Error("future mute -> muted")
	}
}

// thresholdForCond prefers alert threshold, falls back to warn, else invalid.
func TestThresholdForCond(t *testing.T) {
	if got := thresholdForCond(models.Conditions{AlertThreshold: ptr(5), WarnThreshold: ptr(2)}); !got.Valid || got.Float64 != 5 {
		t.Errorf("alert preferred = %+v, want 5", got)
	}
	if got := thresholdForCond(models.Conditions{WarnThreshold: ptr(2)}); !got.Valid || got.Float64 != 2 {
		t.Errorf("warn fallback = %+v, want 2", got)
	}
	if got := thresholdForCond(models.Conditions{}); got.Valid {
		t.Errorf("no threshold -> invalid, got %+v", got)
	}
}

func TestSummarizeScope(t *testing.T) {
	if got := summarizeScope(models.MonitorRow{}); got != "" {
		t.Errorf("empty scope -> empty, got %q", got)
	}
	m := models.MonitorRow{Scope: models.Scope{Tags: []models.ScopeTag{
		{Key: "service", Value: "orders"},
		{Key: "env", Value: "prod"},
	}}}
	if got := summarizeScope(m); got != "service:orders env:prod" {
		t.Errorf("summarizeScope = %q", got)
	}
}

func TestServiceFromScope(t *testing.T) {
	m := models.MonitorRow{Scope: models.Scope{Tags: []models.ScopeTag{
		{Key: "env", Value: "prod"},
		{Key: "service", Value: "orders"},
	}}}
	if got := serviceFromScope(m); got != "orders" {
		t.Errorf("serviceFromScope = %q, want orders", got)
	}
	if got := serviceFromScope(models.MonitorRow{Scope: models.Scope{Tags: []models.ScopeTag{}}}); got != "" {
		t.Errorf("no service tag -> empty, got %q", got)
	}
}

func TestDefaultMessage(t *testing.T) {
	m := models.MonitorRow{Name: "CPU high"}
	if got := defaultMessage(m, 90, 80, expr.Decision{}); got != "CPU high triggered — value 90 vs threshold 80" {
		t.Errorf("triggered msg = %q", got)
	}
	if got := defaultMessage(m, 10, 80, expr.Decision{IsRecovery: true}); got != "CPU high recovered — value 10 vs threshold 80" {
		t.Errorf("recovered msg = %q", got)
	}
}

func TestRenderMessageBody(t *testing.T) {
	d := expr.Decision{NewStatus: "alert"}
	blank := models.MonitorRow{Name: "M", MessageBody: sql.NullString{Valid: true, String: "   "}}
	if got := renderMessageBody(blank, 5, 3, "", d); got != "M triggered — value 5 vs threshold 3" {
		t.Errorf("blank body should default, got %q", got)
	}
	custom := models.MonitorRow{Name: "M", MessageBody: sql.NullString{Valid: true, String: "{{monitor.name}}={{value}}/{{threshold}}"}}
	if got := renderMessageBody(custom, 5, 3, "", d); got != "M=5/3" {
		t.Errorf("custom render = %q, want M=5/3", got)
	}
}

func TestNextEvalOnly(t *testing.T) {
	now := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	m := models.MonitorRow{ID: 7, EvalEverySec: 60}
	got := nextEvalOnly(m, models.MonitorStateRow{Status: ""}, now)
	if got.PrevStatus != "no_data" || got.NewStatus != "no_data" {
		t.Errorf("empty status should be no_data, got prev=%q new=%q", got.PrevStatus, got.NewStatus)
	}
	if !got.NextEvaluationAt.Equal(now.Add(60 * time.Second)) {
		t.Errorf("next eval = %v, want +60s", got.NextEvaluationAt)
	}
	if !got.IncrementEvalCount {
		t.Error("eval count should increment")
	}
}

func TestBuildUpdateArgs_TriggeredAt(t *testing.T) {
	now := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	m := models.MonitorRow{ID: 1, EvalEverySec: 30}
	res := query.ScalarResult{Value: 9, HasData: true}

	fresh := buildUpdateArgs(m, models.MonitorStateRow{Status: "ok"}, expr.Decision{NewStatus: "alert"}, res, now)
	if !fresh.TriggeredAt.Valid || !fresh.TriggeredAt.Time.Equal(now) {
		t.Errorf("new alert TriggeredAt = %+v, want now", fresh.TriggeredAt)
	}
	if !fresh.CurrentValue.Valid || fresh.CurrentValue.Float64 != 9 {
		t.Errorf("CurrentValue = %+v, want 9", fresh.CurrentValue)
	}

	orig := sql.NullTime{Valid: true, Time: now.Add(-time.Hour)}
	cont := buildUpdateArgs(m, models.MonitorStateRow{Status: "alert", TriggeredAt: orig}, expr.Decision{NewStatus: "alert"}, res, now)
	if !cont.TriggeredAt.Time.Equal(orig.Time) {
		t.Errorf("ongoing alert TriggeredAt = %v, want preserved %v", cont.TriggeredAt.Time, orig.Time)
	}

	ok := buildUpdateArgs(m, models.MonitorStateRow{Status: "alert"}, expr.Decision{NewStatus: "ok", ShouldNotify: true}, res, now)
	if ok.TriggeredAt.Valid {
		t.Errorf("ok status should not set TriggeredAt, got %+v", ok.TriggeredAt)
	}
	if !ok.LastNotifiedAt.Valid {
		t.Error("ShouldNotify should set LastNotifiedAt")
	}
}

package stream

import (
	"testing"
	"time"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

func f(v float64) *float64 { return &v }

func TestEngineTransitionsOnlyWhenThresholdStateChanges(t *testing.T) {
	m := models.MonitorRow{ID: 7, TenantID: 2, Active: true, Type: "metric", Query: models.MonitorQuery{Metric: &models.MetricQuery{Metric: "cpu", Aggregation: "avg", WindowSec: 60}}, Conditions: models.Conditions{Comparator: "above", AlertThreshold: f(80)}}
	e := NewEngine([]models.MonitorRow{m}, []models.MonitorStateRow{{MonitorID: 7, Status: "ok"}})
	base := time.Now().UTC().Truncate(time.Second)
	if got := e.OnMetric(MetricEvent{TenantID: 2, MetricName: "cpu", TimestampNs: base.UnixNano(), Value: 90}); len(got) != 1 || got[0].Decision.NewStatus != "alert" {
		t.Fatalf("first event = %#v, want alert transition", got)
	} else {
		e.Commit(got[0])
	}
	if got := e.OnMetric(MetricEvent{TenantID: 2, MetricName: "cpu", TimestampNs: base.Add(time.Second).UnixNano(), Value: 95}); len(got) != 0 {
		t.Fatalf("ongoing alert = %#v, want no transition", got)
	}
	if got := e.OnMetric(MetricEvent{TenantID: 2, MetricName: "cpu", TimestampNs: base.Add(61 * time.Second).UnixNano(), Value: 10}); len(got) != 1 || !got[0].Decision.IsRecovery {
		t.Fatalf("recovery = %#v, want recovery transition", got)
	}
}

func TestEngineUsesSlidingWindowAggregation(t *testing.T) {
	m := models.MonitorRow{ID: 9, TenantID: 2, Active: true, Type: "metric", Query: models.MonitorQuery{Metric: &models.MetricQuery{Metric: "latency", Aggregation: "avg", WindowSec: 60}}, Conditions: models.Conditions{Comparator: "above", AlertThreshold: f(50)}}
	e := NewEngine([]models.MonitorRow{m}, []models.MonitorStateRow{{MonitorID: 9, Status: "ok"}})
	base := time.Now().UTC().Truncate(time.Second)
	first := e.OnMetric(MetricEvent{TenantID: 2, MetricName: "latency", TimestampNs: base.UnixNano(), Value: 40})
	for _, transition := range first {
		e.Commit(transition)
	}
	got := e.OnMetric(MetricEvent{TenantID: 2, MetricName: "latency", TimestampNs: base.Add(time.Second).UnixNano(), Value: 80})
	if len(got) != 1 || got[0].Value != 60 {
		t.Fatalf("window result = %#v, want transition at average 60", got)
	}
}

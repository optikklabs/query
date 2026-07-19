package ingestion

import (
	"testing"
	"time"
)

func TestSplitDailySignals(t *testing.T) {
	day := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	logs, spans, metrics := splitDailySignals([]signalDateCountRow{
		{Signal: "logs", Day: day, Count: 10},
		{Signal: "spans", Day: day, Count: 20},
		{Signal: "metrics", Day: day, Count: 30},
	})
	if logs[0].Count != 10 || spans[0].Count != 20 || metrics[0].Count != 30 {
		t.Fatalf("unexpected split: logs=%v spans=%v metrics=%v", logs, spans, metrics)
	}
}

func TestSplitServiceUsageAggregatesDays(t *testing.T) {
	day := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	usage := splitServiceUsage([]serviceUsageRow{
		{Period: "current", Signal: "logs", Day: day, Service: "api", Env: "prod", Count: 10},
		{Period: "current", Signal: "logs", Day: day.AddDate(0, 0, 1), Service: "api", Count: 20},
		{Period: "prior", Signal: "spans", Day: day, Service: "api", Count: 5},
	})
	if len(usage.logTotals) != 1 || usage.logTotals[0].Count != 30 {
		t.Fatalf("current totals = %v", usage.logTotals)
	}
	if len(usage.priorSpans) != 1 || usage.priorSpans[0].Count != 5 {
		t.Fatalf("prior totals = %v", usage.priorSpans)
	}
	if len(usage.dailyLogs) != 2 {
		t.Fatalf("daily logs = %v", usage.dailyLogs)
	}
}

func TestSummarizeCardinality(t *testing.T) {
	active, top := summarizeCardinality([]metricCardinalityRow{
		{Name: "small", Count: 2},
		{Name: "large", Count: 7},
		{IsTotal: 1, Count: 8},
	})
	if active != 8 || top.Name != "large" || top.Count != 7 {
		t.Fatalf("active=%d top=%+v", active, top)
	}
}

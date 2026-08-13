package timebucket

import (
	"reflect"
	"testing"
	"time"
)

func TestDisplayGrain(t *testing.T) {
	tests := []struct {
		name   string
		window time.Duration
		want   time.Duration
	}{
		{name: "short", window: time.Hour, want: time.Minute},
		{name: "medium", window: 6 * time.Hour, want: 5 * time.Minute},
		{name: "long", window: 48 * time.Hour, want: time.Hour},
		{name: "very long", window: 20 * 24 * time.Hour, want: 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DisplayGrain(tt.window.Milliseconds()); got != tt.want {
				t.Fatalf("DisplayGrain(%s) = %s, want %s", tt.window, got, tt.want)
			}
		})
	}
}

func TestRollupsFollowDisplayGrain(t *testing.T) {
	startMs := int64(1_700_000_000_000)
	tests := []struct {
		window  time.Duration
		grain   int64
		metrics string
		spans   string
	}{
		{window: time.Hour, grain: 60, metrics: "optikk.metrics_1m_v2", spans: "optikk.span_stats_1m"},
		{window: 6 * time.Hour, grain: 300, metrics: "optikk.metrics_5m_v2", spans: "optikk.span_stats_5m"},
		{window: 48 * time.Hour, grain: 3600, metrics: "optikk.metrics_1h_v2", spans: "optikk.span_stats_1h"},
	}
	for _, tt := range tests {
		endMs := startMs + tt.window.Milliseconds()
		if got := RollupGrainSeconds(tt.window.Milliseconds()); got != tt.grain {
			t.Errorf("RollupGrainSeconds(%s) = %d, want %d", tt.window, got, tt.grain)
		}
		if got := MetricsRollup(startMs, endMs); got != tt.metrics {
			t.Errorf("MetricsRollup(%s) = %q, want %q", tt.window, got, tt.metrics)
		}
		if got := SpanStatsRollup(startMs, endMs); got != tt.spans {
			t.Errorf("SpanStatsRollup(%s) = %q, want %q", tt.window, got, tt.spans)
		}
	}
}

func TestBuildDenseTimestampsFloorsStartAndExcludesEnd(t *testing.T) {
	got := BuildDenseTimestamps(61_000, 181_000, 60)
	want := []int64{60_000, 120_000, 180_000}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildDenseTimestamps() = %v, want %v", got, want)
	}
}

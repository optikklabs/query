package logtrends

import (
	"testing"
	"time"
)

// mapTrend formats the bucket timestamp as UTC and copies the per-severity
// counts through unchanged.
func TestMapTrend(t *testing.T) {
	rows := []TrendRow{
		{TimeBucket: time.Date(2026, 6, 24, 9, 5, 0, 0, time.UTC), Total: 10, Error: 1, Warn: 2, Info: 6, Debug: 1},
	}
	out := mapTrend(rows)
	if len(out) != 1 {
		t.Fatalf("got %d buckets, want 1", len(out))
	}
	b := out[0]
	if b.TimeBucket != "2026-06-24 09:05:00" {
		t.Errorf("TimeBucket = %q, want 2026-06-24 09:05:00", b.TimeBucket)
	}
	if b.Total != 10 || b.Error != 1 || b.Warn != 2 || b.Info != 6 || b.Debug != 1 {
		t.Errorf("counts not copied through: %+v", b)
	}
}

func TestMapTrend_Empty(t *testing.T) {
	if got := mapTrend(nil); len(got) != 0 {
		t.Errorf("want empty, got %+v", got)
	}
}

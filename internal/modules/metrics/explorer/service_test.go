package explorer

import (
	"testing"
	"time"
)

// For a cumulative counter, row.Sum already carries the per-bucket increase.
// rate must scale it per second; every other aggregation returns the increase.
func TestApplyAggregationCumulative(t *testing.T) {
	rows := []timeseriesPointDTO{
		{BucketAt: time.Unix(0, 0), Sum: 120, Count: 120},
	}
	// 2h window, default grain -> 60s buckets.
	startMs := int64(0)
	endMs := int64(2 * 60 * 60 * 1000)

	cases := []struct {
		agg  string
		want float64
	}{
		{"sum", 120},
		{"rate", 2}, // 120 / 60s
		{"avg", 120},
		{"count", 120},
		{"max", 120},
	}
	for _, c := range cases {
		got := applyAggregation(rows, c.agg, startMs, endMs, "", true)
		if len(got) != 1 || got[0].Value != c.want {
			t.Errorf("cumulative agg %q = %v, want %v", c.agg, got, c.want)
		}
	}
}

// Delta/gauge path is unchanged: rate divides the summed delta by the bucket.
func TestApplyAggregationDelta(t *testing.T) {
	rows := []timeseriesPointDTO{
		{BucketAt: time.Unix(0, 0), Sum: 120, Count: 4, Min: 1, Max: 50},
	}
	startMs := int64(0)
	endMs := int64(2 * 60 * 60 * 1000)

	cases := []struct {
		agg  string
		want float64
	}{
		{"sum", 120},
		{"rate", 2},   // 120 / 60s
		{"avg", 30},   // 120 / 4
		{"min", 1},
		{"max", 50},
		{"count", 4},
	}
	for _, c := range cases {
		got := applyAggregation(rows, c.agg, startMs, endMs, "", false)
		if len(got) != 1 || got[0].Value != c.want {
			t.Errorf("delta agg %q = %v, want %v", c.agg, got, c.want)
		}
	}
}

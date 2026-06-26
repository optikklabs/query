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

	startMs := int64(0)
	endMs := int64(2 * 60 * 60 * 1000)

	cases := []struct {
		agg  string
		want float64
	}{
		{"sum", 120},
		{"rate", 2},
		{"avg", 120},
		{"count", 120},
		{"max", 120},
	}
	for _, c := range cases {
		got := applyAggregation(rows, c.agg, startMs, endMs, "", true, false)
		if len(got) != 1 || got[0].Value != c.want {
			t.Errorf("cumulative agg %q = %v, want %v", c.agg, got, c.want)
		}
	}
}

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
		{"rate", 2},
		{"avg", 30},
		{"min", 1},
		{"max", 50},
		{"count", 4},
	}
	for _, c := range cases {
		got := applyAggregation(rows, c.agg, startMs, endMs, "", false, false)
		if len(got) != 1 || got[0].Value != c.want {
			t.Errorf("delta agg %q = %v, want %v", c.agg, got, c.want)
		}
	}
}

func TestApplyAggregationHistogram(t *testing.T) {
	rows := []timeseriesPointDTO{{
		BucketAt: time.Unix(0, 0),
		Sum:      0, Count: 3,
		HistSum: 500, HistCount: 100,
		Quantiles: []float64{10, 95, 99},
	}}
	startMs := int64(0)
	endMs := int64(2 * 60 * 60 * 1000)

	cases := []struct {
		agg  string
		want float64
	}{
		{"count", 100},
		{"sum", 500},
		{"avg", 5},
		{"rate", 100.0 / 60},
		{"p50", 10},
		{"p95", 95},
		{"p99", 99},
		{"p75", 0},
	}
	for _, c := range cases {
		got := applyAggregation(rows, c.agg, startMs, endMs, "", false, true)
		if len(got) != 1 || got[0].Value != c.want {
			t.Errorf("histogram agg %q = %v, want %v", c.agg, got, c.want)
		}
	}
}

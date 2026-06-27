package filter

import (
	"testing"
	"time"
)

type rateRow struct {
	ts  time.Time
	dim string
	val float64
}

// FoldCounterRateByDim divides the per-bucket delta sum by the display-grain
// seconds. These cases pin the two grains that mattered in the historical bugs.
func TestFoldCounterRateByDim_RatePerSec(t *testing.T) {
	base := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	ts := func(d time.Duration) time.Time { return base.Add(d) }
	tsOf := func(r rateRow) time.Time { return r.ts }
	dimOf := func(r rateRow) string { return r.dim }
	valOf := func(r rateRow) float64 { return r.val }

	cases := []struct {
		name      string
		rows      []rateRow
		windowMs  int64
		wantRate  float64
		wantCount int
	}{
		{

			name:      "60s grain sums delta over bucket seconds",
			rows:      []rateRow{{ts(10 * time.Second), "orders", 40}, {ts(30 * time.Second), "orders", 34}},
			windowMs:  20 * 60 * 1000,
			wantRate:  74.0 / 60.0,
			wantCount: 1,
		},
		{

			name:      "300s grain divides by 5m seconds",
			rows:      []rateRow{{ts(10 * time.Second), "orders", 372}},
			windowMs:  6 * 60 * 60 * 1000,
			wantRate:  372.0 / 300.0,
			wantCount: 1,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := FoldCounterRateByDim(c.rows, tsOf, dimOf, valOf, 0, c.windowMs)
			if len(out) != c.wantCount {
				t.Fatalf("got %d folds, want %d: %+v", len(out), c.wantCount, out)
			}
			if out[0].Rate != c.wantRate {
				t.Errorf("rate = %v, want %v", out[0].Rate, c.wantRate)
			}
		})
	}
}

func TestFoldCounterRateByDim_SeparatesBuckets(t *testing.T) {
	base := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	rows := []rateRow{
		{base, "orders", 60},
		{base.Add(90 * time.Second), "orders", 120},
	}
	out := FoldCounterRateByDim(rows,
		func(r rateRow) time.Time { return r.ts },
		func(r rateRow) string { return r.dim },
		func(r rateRow) float64 { return r.val },
		0, 20*60*1000)
	if len(out) != 2 {
		t.Fatalf("got %d folds, want 2 (distinct buckets): %+v", len(out), out)
	}
	if out[0].Rate != 1 || out[1].Rate != 2 {
		t.Errorf("rates = %v,%v want 1,2", out[0].Rate, out[1].Rate)
	}
}

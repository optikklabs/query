package redfleet

import (
	"math"
	"testing"
	"time"
)

// denseBuckets must cover every grain-aligned bucket in [start, end] so quiet
// buckets become explicit points instead of collapsing the chart axis.
func TestDenseBucketsCoversWindow(t *testing.T) {
	start := time.Date(2026, 6, 22, 10, 0, 30, 0, time.UTC).UnixMilli()
	end := time.Date(2026, 6, 22, 10, 4, 10, 0, time.UTC).UnixMilli()

	got := denseBuckets(start, end, time.Minute)

	want := []time.Time{
		time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 22, 10, 1, 0, 0, time.UTC),
		time.Date(2026, 6, 22, 10, 2, 0, 0, time.UTC),
		time.Date(2026, 6, 22, 10, 3, 0, 0, time.UTC),
		time.Date(2026, 6, 22, 10, 4, 0, 0, time.UTC),
	}
	if len(got) != len(want) {
		t.Fatalf("bucket count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if !got[i].Equal(want[i]) {
			t.Errorf("bucket[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

// normalizeUtilization scales fractions (<=1) to percent, keeps 1..100 as-is,
// and drops invalid or out-of-range values.
func TestNormalizeUtilization(t *testing.T) {
	cases := []struct {
		in      float64
		wantNil bool
		want    float64
	}{
		{0.5, false, 50},
		{1.0, false, 100},
		{50, false, 50},
		{100, false, 100},
		{100.1, true, 0},
		{-1, true, 0},
		{math.NaN(), true, 0},
		{math.Inf(1), true, 0},
	}
	for _, c := range cases {
		got := normalizeUtilization(c.in)
		if c.wantNil {
			if got != nil {
				t.Errorf("normalizeUtilization(%v) = %v, want nil", c.in, *got)
			}
			continue
		}
		if got == nil || *got != c.want {
			t.Errorf("normalizeUtilization(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestAverageFloats(t *testing.T) {
	if got := averageFloats(nil); got != nil {
		t.Errorf("empty input must return nil, got %v", *got)
	}
	if got := averageFloats([]float64{10, 20, 30}); got == nil || *got != 20 {
		t.Errorf("averageFloats([10,20,30]) = %v, want 20", got)
	}
}


package redfleet

import (
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

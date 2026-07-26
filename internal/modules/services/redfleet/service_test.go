package redfleet

import (
	"testing"
	"time"

	"github.com/optikklabs/query/internal/infra/timebucket"
)

// denseBuckets must cover every grain-aligned bucket in [start, end] so quiet
// buckets become explicit points instead of collapsing the chart axis.
func TestDenseBucketsCoversWindow(t *testing.T) {
	start := time.Date(2026, 6, 22, 10, 0, 30, 0, time.UTC).UnixMilli()
	end := time.Date(2026, 6, 22, 10, 4, 10, 0, time.UTC).UnixMilli()

	got := timebucket.DenseBuckets(start, end, time.Minute)

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

func TestFleetTotalsUseMergedHistogramRow(t *testing.T) {
	rows := []redMetricsRow{
		{ServiceName: "api", TotalCount: 90, ErrorCount: 9, P99Ms: 100},
		{ServiceName: "worker", TotalCount: 10, ErrorCount: 1, P99Ms: 1000},
		{IsTotal: 1, TotalCount: 100, ErrorCount: 10, P50Ms: 20, P95Ms: 80, P99Ms: 900},
	}
	services := mapFleetServices(rows)
	totals := computeFleetTotals(fleetTotalRow(rows), len(services), 0, 10_000)

	if totals.ServiceCount != 2 || totals.TotalSpanCount != 100 || totals.TotalErrors != 10 {
		t.Fatalf("totals = %+v", totals)
	}
	if totals.AvgP99Ms != 900 {
		t.Fatalf("fleet p99 = %v, want merged value 900", totals.AvgP99Ms)
	}
}

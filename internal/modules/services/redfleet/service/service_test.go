package service

import (
	"testing"
	"time"

	"github.com/optikklabs/query/internal/modules/services/redfleet/filter"
	"github.com/optikklabs/query/internal/modules/services/redfleet/models"
)

func TestBuildEndpointRateSeries(t *testing.T) {
	start := time.Now().UTC().Truncate(time.Minute)
	f := filter.Filters{StartMs: start.UnixMilli(), EndMs: start.Add(2 * time.Minute).UnixMilli()}
	rows := []models.EndpointRateRow{
		{BucketAt: start, OperationName: "smaller", RequestCount: 10, ErrorCount: 1},
		{BucketAt: start, OperationName: "busiest", RequestCount: 20, ErrorCount: 2},
	}

	got := buildEndpointRateSeries(rows, f, 1)
	if len(got.Series) != 1 || got.Series[0].OperationName != "busiest" {
		t.Fatalf("top series = %#v, want busiest only", got.Series)
	}
	if got.Totals.RequestCount[0] != 30 {
		t.Fatalf("total request count = %d, want 30", got.Totals.RequestCount[0])
	}
	if len(got.Timestamps) != 2 || got.Timestamps[0] != start.UnixMilli() {
		t.Fatalf("timestamps = %v", got.Timestamps)
	}
	if got.Series[0].ErrorRate[1] != nil || got.Totals.ErrorRate[1] != nil {
		t.Fatal("empty buckets must preserve null rates")
	}
}

func TestTopEndpointRowsUsesDeterministicTieBreak(t *testing.T) {
	now := time.Now()
	rows := []models.EndpointRateRow{
		{BucketAt: now, OperationName: "z", RequestCount: 1},
		{BucketAt: now, OperationName: "a", RequestCount: 1},
	}
	got := topEndpointRows(rows, 1)
	if len(got) != 1 || got[0].OperationName != "a" {
		t.Fatalf("topEndpointRows = %#v, want operation a", got)
	}
}

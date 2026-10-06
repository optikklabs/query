package ingestion

import (
	"slices"
	"testing"
	"time"
)

func TestDateAxisCoversEveryDayInclusive(t *testing.T) {
	start := time.Date(2026, 3, 30, 22, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 1, 1, 0, 0, 0, time.UTC)
	axis := newDateAxis(start.UnixMilli(), end.UnixMilli())

	want := []string{"2026-03-30", "2026-03-31", "2026-04-01"}
	if !slices.Equal(axis.dates, want) {
		t.Fatalf("dates = %v, want %v", axis.dates, want)
	}

	counts, bytes := axis.fill([]dateCountRow{
		{Day: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), Count: 2, Bytes: 20},
		{Day: time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC), Count: 1, Bytes: 10},
		{Day: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Count: 9, Bytes: 90},
	})
	if !slices.Equal(counts, []uint64{0, 3, 0}) || !slices.Equal(bytes, []uint64{0, 30, 0}) {
		t.Fatalf("fill = %v, %v", counts, bytes)
	}
}

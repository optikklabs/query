package tracewindow

import (
	"testing"
	"time"
)

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestDaysRoundsOutToPartitionBoundaries(t *testing.T) {
	tests := []struct {
		name               string
		w                  Window
		wantStart, wantEnd string
	}{
		{
			name:      "within one day",
			w:         Window{Start: ts("2026-07-16T08:54:10.4Z"), End: ts("2026-07-16T08:54:10.9Z")},
			wantStart: "2026-07-16T00:00:00Z",
			wantEnd:   "2026-07-17T00:00:00Z",
		},
		{
			name:      "straddles midnight",
			w:         Window{Start: ts("2026-07-16T23:59:59Z"), End: ts("2026-07-17T00:00:01Z")},
			wantStart: "2026-07-16T00:00:00Z",
			wantEnd:   "2026-07-18T00:00:00Z",
		},
		{
			name:      "already on a boundary still covers its own day",
			w:         Window{Start: ts("2026-07-16T00:00:00Z"), End: ts("2026-07-16T00:00:00Z")},
			wantStart: "2026-07-16T00:00:00Z",
			wantEnd:   "2026-07-17T00:00:00Z",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.w.Days()
			if !got.Start.Equal(ts(tt.wantStart)) {
				t.Errorf("Start = %v, want %v", got.Start, tt.wantStart)
			}
			if !got.End.Equal(ts(tt.wantEnd)) {
				t.Errorf("End = %v, want %v", got.End, tt.wantEnd)
			}
			// A widened window must never lose the span lifetime it came from.
			if got.Start.After(tt.w.Start) || got.End.Before(tt.w.End) {
				t.Errorf("Days() narrowed the window: %v..%v does not contain %v..%v",
					got.Start, got.End, tt.w.Start, tt.w.End)
			}
		})
	}
}

func TestClamp(t *testing.T) {
	w := Window{Start: ts("2026-07-16T10:00:00Z"), End: ts("2026-07-16T11:00:00Z")}
	uiStart := ts("2026-07-16T00:00:00Z").UnixMilli()
	uiEnd := ts("2026-07-17T00:00:00Z").UnixMilli()

	t.Run("tightens a wider ui range to the trace", func(t *testing.T) {
		s, e := w.Clamp(uiStart, uiEnd)
		if s != w.Start.UnixMilli() || e != w.End.UnixMilli() {
			t.Fatalf("Clamp() = %d..%d, want %d..%d", s, e, w.Start.UnixMilli(), w.End.UnixMilli())
		}
	})

	t.Run("never widens a narrower ui range", func(t *testing.T) {
		narrowStart := ts("2026-07-16T10:30:00Z").UnixMilli()
		narrowEnd := ts("2026-07-16T10:40:00Z").UnixMilli()
		s, e := w.Clamp(narrowStart, narrowEnd)
		if s != narrowStart || e != narrowEnd {
			t.Fatalf("Clamp() = %d..%d, want the caller range %d..%d", s, e, narrowStart, narrowEnd)
		}
	})

	t.Run("disjoint ranges produce an empty intersection", func(t *testing.T) {
		otherDayStart := ts("2026-07-20T00:00:00Z").UnixMilli()
		otherDayEnd := ts("2026-07-21T00:00:00Z").UnixMilli()
		s, e := w.Clamp(otherDayStart, otherDayEnd)
		if s <= e {
			t.Fatalf("Clamp() = %d..%d, want start > end for a disjoint range", s, e)
		}
	})
}

func TestRetentionFallbackSpansRetention(t *testing.T) {
	now := ts("2026-07-19T12:00:00Z")
	w := RetentionFallback(now)
	if !w.End.Equal(now) {
		t.Errorf("End = %v, want %v", w.End, now)
	}
	if got := w.End.Sub(w.Start); got != logsRetention {
		t.Errorf("window width = %v, want %v", got, logsRetention)
	}
}

package service

import (
	"testing"
	"time"

	"github.com/optikklabs/query/internal/modules/traces/models"
	"github.com/optikklabs/query/internal/modules/traces/repository"
)

func TestNormalizeDBStatement(t *testing.T) {
	cases := map[string]string{
		"":                                   "",
		"SELECT * FROM t WHERE id = 42":      "SELECT * FROM t WHERE id = ?",
		"SELECT * FROM t WHERE name = 'bob'": "SELECT * FROM t WHERE name = ?",
		"SELECT   *    FROM   t":             "SELECT * FROM t",
	}
	for in, want := range cases {
		if got := normalizeDBStatement(in); got != want {
			t.Errorf("normalizeDBStatement(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFillStartNs(t *testing.T) {
	ts := time.Unix(1, 500)
	items := []models.SpanListItem{{Timestamp: ts}}
	fillStartNs(items)
	if items[0].StartNs != ts.UnixNano() {
		t.Errorf("StartNs = %d, want %d", items[0].StartNs, ts.UnixNano())
	}
}

// Events fan out per span; exceptions are collected only when typed, and the
// exception slice is reversed (newest-first from the ascending input).
func TestSplitEventRows(t *testing.T) {
	rows := []repository.SpanEventCombinedRow{
		{SpanID: "s1", Events: []repository.SpanEventTuple{{Name: "a"}, {Name: "b"}}, ExceptionType: "E1"},
		{SpanID: "s2", Events: nil, ExceptionType: ""},
		{SpanID: "s3", Events: []repository.SpanEventTuple{{Name: "c"}}, ExceptionType: "E2"},
	}
	events, exceptions := splitEventRows(rows)
	if len(events) != 3 {
		t.Errorf("got %d events, want 3", len(events))
	}
	if len(exceptions) != 2 {
		t.Fatalf("got %d exceptions, want 2", len(exceptions))
	}
	if exceptions[0].ExceptionType != "E2" || exceptions[1].ExceptionType != "E1" {
		t.Errorf("exceptions not reversed: %+v", exceptions)
	}
}

// The duration is derived from the first and last span, not read from a
// column, so it is the one part of the summary the fold owns.
func TestFoldTraceSummaryDerivesDuration(t *testing.T) {
	start := time.Unix(100, 0)
	got := foldTraceSummary(repository.TraceSummaryRow{
		TraceID:   "t1",
		StartTime: start,
		EndTime:   start.Add(250 * time.Millisecond),
		SpanCount: 4,
	})
	if got.DurationMs != 250 {
		t.Errorf("DurationMs = %v, want 250", got.DurationMs)
	}
	if got.StartMs != uint64(start.UnixMilli()) {
		t.Errorf("StartMs = %d, want %d", got.StartMs, start.UnixMilli())
	}
	if got.SpanCount != 4 {
		t.Errorf("SpanCount = %d, want 4", got.SpanCount)
	}
}

package detail

import (
	"testing"
	"time"
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
	items := []SpanListItem{{Timestamp: ts}}
	fillStartNs(items)
	if items[0].StartNs != ts.UnixNano() {
		t.Errorf("StartNs = %d, want %d", items[0].StartNs, ts.UnixNano())
	}
}

// Events fan out per span; exceptions are collected only when typed, and the
// exception slice is reversed (newest-first from the ascending input).
func TestSplitEventRows(t *testing.T) {
	rows := []spanEventCombinedRow{
		{SpanID: "s1", Events: []spanEventTuple{{Name: "a"}, {Name: "b"}}, ExceptionType: "E1"},
		{SpanID: "s2", Events: nil, ExceptionType: ""},
		{SpanID: "s3", Events: []spanEventTuple{{Name: "c"}}, ExceptionType: "E2"},
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

package detail

import (
	"testing"
	"time"
)

func TestFlattenJSONAttrs(t *testing.T) {
	t.Run("empty forms return nil", func(t *testing.T) {
		for _, in := range []string{"", "  ", "{}", "null"} {
			if got := flattenJSONAttrs(in); got != nil {
				t.Errorf("flattenJSONAttrs(%q) = %v, want nil", in, got)
			}
		}
	})
	t.Run("invalid json returns nil", func(t *testing.T) {
		if got := flattenJSONAttrs("{not json"); got != nil {
			t.Errorf("want nil for invalid json, got %v", got)
		}
	})
	t.Run("nested keys are dotted, types stringified", func(t *testing.T) {
		got := flattenJSONAttrs(`{"http":{"status":200,"ok":true},"name":"x","tags":["a","b"],"empty":null}`)
		want := map[string]string{
			"http.status": "200",
			"http.ok":     "true",
			"name":        "x",
			"tags.0":      "a",
			"tags.1":      "b",
			"empty":       "",
		}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("key %q = %q, want %q", k, got[k], v)
			}
		}
	})
}

func TestParseSpanLinks(t *testing.T) {
	t.Run("empty forms return nil", func(t *testing.T) {
		for _, in := range []string{"", "  ", "[]"} {
			if got := parseSpanLinks(in); got != nil {
				t.Errorf("parseSpanLinks(%q) = %v, want nil", in, got)
			}
		}
	})
	t.Run("invalid json returns nil", func(t *testing.T) {
		if got := parseSpanLinks("[broken"); got != nil {
			t.Errorf("want nil, got %v", got)
		}
	})
	t.Run("maps wire fields", func(t *testing.T) {
		got := parseSpanLinks(`[{"traceId":"t1","spanId":"s1","traceState":"k=v","attributes":{"a":"b"}}]`)
		if len(got) != 1 {
			t.Fatalf("got %d links, want 1", len(got))
		}
		l := got[0]
		if l.TraceID != "t1" || l.SpanID != "s1" || l.TraceState != "k=v" || l.Attributes["a"] != "b" {
			t.Errorf("link = %+v", l)
		}
	})
}

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

func TestParseEventJSON(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		n, a := parseEventJSON("")
		if n != "" || a != "{}" {
			t.Errorf("got %q,%q want \"\",{}", n, a)
		}
	})
	t.Run("non-object treated as name", func(t *testing.T) {
		n, a := parseEventJSON("plain event")
		if n != "plain event" || a != "{}" {
			t.Errorf("got %q,%q", n, a)
		}
	})
	t.Run("object with attrs", func(t *testing.T) {
		n, a := parseEventJSON(`{"name":"ev","attributes":{"k":"v"}}`)
		if n != "ev" || a != `{"k":"v"}` {
			t.Errorf("got %q,%q want ev,{\"k\":\"v\"}", n, a)
		}
	})
	t.Run("object without attrs", func(t *testing.T) {
		n, a := parseEventJSON(`{"name":"ev"}`)
		if n != "ev" || a != "{}" {
			t.Errorf("got %q,%q", n, a)
		}
	})
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
		{SpanID: "s1", Events: []string{"a", "b"}, ExceptionType: "E1"},
		{SpanID: "s2", Events: nil, ExceptionType: ""},
		{SpanID: "s3", Events: []string{"c"}, ExceptionType: "E2"},
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

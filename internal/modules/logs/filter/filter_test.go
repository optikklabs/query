package filter

import (
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

func namedArgs(args []any) map[string]any {
	m := map[string]any{}
	for _, a := range args {
		if nv, ok := a.(driver.NamedValue); ok {
			m[nv.Name] = nv.Value
		}
	}
	return m
}

func TestValidate(t *testing.T) {
	t.Run("missing start", func(t *testing.T) {
		if err := (&Filters{EndMs: 1000}).Validate(); err == nil {
			t.Error("want error for missing startTime")
		}
	})
	t.Run("end before start", func(t *testing.T) {
		if err := (&Filters{StartMs: 2000, EndMs: 1000}).Validate(); err == nil {
			t.Error("want error for end <= start")
		}
	})
	t.Run("clamps over-long window", func(t *testing.T) {
		end := time.Now().UnixMilli()
		f := Filters{StartMs: end - 2*maxTimeRangeMs, EndMs: end}
		if err := f.Validate(); err != nil {
			t.Fatal(err)
		}
		if f.EndMs-f.StartMs != maxTimeRangeMs {
			t.Errorf("window = %d, want clamped to %d", f.EndMs-f.StartMs, maxTimeRangeMs)
		}
	})
	t.Run("defaults search mode", func(t *testing.T) {
		f := Filters{StartMs: 1, EndMs: 2}
		if err := f.Validate(); err != nil {
			t.Fatal(err)
		}
		if f.SearchMode != "ngram" {
			t.Errorf("SearchMode = %q, want ngram", f.SearchMode)
		}
	})
}

func TestBuildClauses_Base(t *testing.T) {
	rw, w, args := BuildClauses(Filters{TeamID: 1, StartMs: 1000, EndMs: 2000})
	if rw != "" || w != "" {
		t.Errorf("base clauses should be empty, got rw=%q w=%q", rw, w)
	}
	if got := len(namedArgs(args)); got != 3 {
		t.Errorf("got %d args, want 3 base args", got)
	}
}

// Resource dims (service/host/pod/container/env) go to resourceWhere; severity,
// trace/span ids and search go to where.
func TestBuildClauses_ResourceVsSpanSplit(t *testing.T) {
	rw, w, _ := BuildClauses(Filters{
		StartMs: 1, EndMs: 2,
		Services:   []string{"svc"},
		Hosts:      []string{"h1"},
		Severities: []string{"ERROR"},
		SpanID:     "abc",
	})
	for _, want := range []string{"service IN @services", "host IN @hosts"} {
		if !strings.Contains(rw, want) {
			t.Errorf("resourceWhere missing %q: %q", want, rw)
		}
	}
	for _, want := range []string{"severity_text IN @severities", "span_id = @spanID"} {
		if !strings.Contains(w, want) {
			t.Errorf("where missing %q: %q", want, w)
		}
	}
}

func TestBuildClauses_Search(t *testing.T) {
	_, exact, _ := BuildClauses(Filters{StartMs: 1, EndMs: 2, Search: "x", SearchMode: "exact"})
	if !strings.Contains(exact, "lower(body) LIKE concat('%', lower(@search), '%')") {
		t.Errorf("exact search clause wrong: %q", exact)
	}
	_, ng, _ := BuildClauses(Filters{StartMs: 1, EndMs: 2, Search: "x", SearchMode: "ngram"})
	if !strings.Contains(ng, "hasToken(body, lower(@search))") {
		t.Errorf("ngram search clause wrong: %q", ng)
	}
}

func TestBuildClauses_AttributeOps(t *testing.T) {
	cases := []struct {
		op   string
		want string
	}{
		{"", "attributes_string[@akey_0] = @aval_0"},
		{"neq", "attributes_string[@akey_0] != @aval_0"},
		{"contains", "positionCaseInsensitive(attributes_string[@akey_0], @aval_0) > 0"},
		{"regex", "match(attributes_string[@akey_0], @aval_0)"},
	}
	for _, c := range cases {
		t.Run(c.op, func(t *testing.T) {
			_, w, args := BuildClauses(Filters{
				StartMs: 1, EndMs: 2,
				Attributes: []AttrFilter{{Key: "k", Op: c.op, Value: "v"}},
			})
			if !strings.Contains(w, c.want) {
				t.Errorf("op %q -> %q, want substring %q", c.op, w, c.want)
			}
			m := namedArgs(args)
			if m["akey_0"] != "k" || m["aval_0"] != "v" {
				t.Errorf("attr binds = %v, want k/v", m)
			}
		})
	}
}

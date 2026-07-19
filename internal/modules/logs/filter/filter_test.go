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
}

func TestValidateAttrs(t *testing.T) {
	valid := func(af AttrFilter) error {
		f := Filters{StartMs: 1, EndMs: 2, Attributes: []AttrFilter{af}}
		return f.Validate()
	}
	t.Run("unknown op rejected", func(t *testing.T) {
		if err := valid(AttrFilter{Key: "k", Op: "like", Value: "v"}); err == nil {
			t.Error("want error for unknown op")
		}
	})
	t.Run("empty key rejected", func(t *testing.T) {
		if err := valid(AttrFilter{Key: " ", Op: "eq", Value: "v"}); err == nil {
			t.Error("want error for empty key")
		}
	})
	t.Run("comparison requires numeric value", func(t *testing.T) {
		if err := valid(AttrFilter{Key: "k", Op: "gte", Value: "abc"}); err == nil {
			t.Error("want error for non-numeric comparison value")
		}
		if err := valid(AttrFilter{Key: "k", Op: "gte", Value: "500"}); err != nil {
			t.Errorf("numeric comparison should pass: %v", err)
		}
	})
	t.Run("invalid regex rejected", func(t *testing.T) {
		if err := valid(AttrFilter{Key: "k", Op: "regex", Value: "("}); err == nil {
			t.Error("want error for invalid regex")
		}
		if err := valid(AttrFilter{Key: "k", Op: "regex", Value: "err.*"}); err != nil {
			t.Errorf("valid regex should pass: %v", err)
		}
	})
	t.Run("all supported ops pass", func(t *testing.T) {
		for _, op := range []string{"", "eq", "neq", "contains", "exists", "not_exists"} {
			if err := valid(AttrFilter{Key: "k", Op: op, Value: "v"}); err != nil {
				t.Errorf("op %q should pass: %v", op, err)
			}
		}
	})
}

func TestBuildClauses_Base(t *testing.T) {
	rw, w, args := BuildClauses(Filters{TenantID: 1, StartMs: 1000, EndMs: 2000})
	wantClause := " AND ts_bucket BETWEEN @startBucket AND @endBucket"
	if rw != "" || w != wantClause {
		t.Errorf("base clauses wrong, got rw=%q w=%q", rw, w)
	}
	if got := len(namedArgs(args)); got != 5 {
		t.Errorf("got %d args, want 5 base args", got)
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
	for _, want := range []string{"upper(severity_text) IN @severities", "span_id = @spanID"} {
		if !strings.Contains(w, want) {
			t.Errorf("where missing %q: %q", want, w)
		}
	}
}

// Severity matching is case-insensitive: the column is uppercased in SQL and
// the incoming values are uppercased before binding.
func TestBuildClauses_SeverityCaseInsensitive(t *testing.T) {
	_, w, args := BuildClauses(Filters{
		StartMs: 1, EndMs: 2,
		Severities:        []string{"error", "Warn"},
		ExcludeSeverities: []string{"info"},
	})
	if !strings.Contains(w, "upper(severity_text) IN @severities") ||
		!strings.Contains(w, "upper(severity_text) NOT IN @excSeverities") {
		t.Errorf("severity clauses wrong: %q", w)
	}
	m := namedArgs(args)
	sev := m["severities"].([]string)
	if sev[0] != "ERROR" || sev[1] != "WARN" {
		t.Errorf("severities not uppercased: %v", sev)
	}
	if exc := m["excSeverities"].([]string); exc[0] != "INFO" {
		t.Errorf("excSeverities not uppercased: %v", exc)
	}
}

// Body search is always case-insensitive substring; the legacy searchMode
// field no longer changes semantics.
func TestBuildClauses_Search(t *testing.T) {
	for _, mode := range []string{"", "exact", "ngram"} {
		_, w, _ := BuildClauses(Filters{StartMs: 1, EndMs: 2, Search: "x", SearchMode: mode})
		if !strings.Contains(w, "positionCaseInsensitive(body, @search) > 0") {
			t.Errorf("mode %q search clause wrong: %q", mode, w)
		}
	}
}

func TestBuildClauses_AttributeOps(t *testing.T) {
	cases := []struct {
		name  string
		attr  AttrFilter
		want  string
		binds map[string]any
	}{
		{
			name:  "eq string",
			attr:  AttrFilter{Key: "k", Op: "eq", Value: "v"},
			want:  " AND attributes_string[@akey_0] = @aval_0",
			binds: map[string]any{"akey_0": "k", "aval_0": "v"},
		},
		{
			name: "eq numeric checks string and number maps",
			attr: AttrFilter{Key: "k", Op: "", Value: "200"},
			want: "(attributes_string[@akey_0] = @aval_0 OR (mapContains(attributes_number, @akey_0)" +
				" AND attributes_number[@akey_0] = @aval_0_n))",
			binds: map[string]any{"aval_0": "200", "aval_0_n": 200.0},
		},
		{
			name: "eq bool checks string and bool maps",
			attr: AttrFilter{Key: "k", Op: "eq", Value: "true"},
			want: "(attributes_string[@akey_0] = @aval_0 OR (mapContains(attributes_bool, @akey_0)" +
				" AND attributes_bool[@akey_0] = @aval_0_b))",
			binds: map[string]any{"aval_0": "true", "aval_0_b": true},
		},
		{
			name: "neq requires key to exist",
			attr: AttrFilter{Key: "k", Op: "neq", Value: "v"},
			want: "(mapContains(attributes_string, @akey_0) AND attributes_string[@akey_0] != @aval_0)",
		},
		{
			name: "contains",
			attr: AttrFilter{Key: "k", Op: "contains", Value: "v"},
			want: "positionCaseInsensitive(attributes_string[@akey_0], @aval_0) > 0",
		},
		{
			name: "regex",
			attr: AttrFilter{Key: "k", Op: "regex", Value: "v.*"},
			want: "match(attributes_string[@akey_0], @aval_0)",
		},
		{
			name: "gte coalesces string and number maps",
			attr: AttrFilter{Key: "k", Op: "gte", Value: "500"},
			want: "coalesce(toFloat64OrNull(attributes_string[@akey_0])," +
				" if(mapContains(attributes_number, @akey_0), attributes_number[@akey_0], NULL)) >= @aval_0",
			binds: map[string]any{"aval_0": 500.0},
		},
		{
			name: "lt",
			attr: AttrFilter{Key: "k", Op: "lt", Value: "10"},
			want: "NULL)) < @aval_0",
		},
		{
			name: "exists spans all maps",
			attr: AttrFilter{Key: "k", Op: "exists"},
			want: " AND (mapContains(attributes_string, @akey_0) OR mapContains(attributes_number, @akey_0)" +
				" OR mapContains(attributes_bool, @akey_0))",
		},
		{
			name: "not_exists",
			attr: AttrFilter{Key: "k", Op: "not_exists"},
			want: " AND NOT (mapContains(attributes_string, @akey_0)",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, w, args := BuildClauses(Filters{
				StartMs: 1, EndMs: 2,
				Attributes: []AttrFilter{c.attr},
			})
			if !strings.Contains(w, c.want) {
				t.Errorf("got %q, want substring %q", w, c.want)
			}
			m := namedArgs(args)
			for k, v := range c.binds {
				if m[k] != v {
					t.Errorf("bind %q = %v, want %v", k, m[k], v)
				}
			}
		})
	}
}

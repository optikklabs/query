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
		f := Filters{EndMs: 1000}
		if err := f.Validate(); err == nil {
			t.Error("want error for missing startTime")
		}
	})
	t.Run("end before start", func(t *testing.T) {
		f := Filters{StartMs: 2000, EndMs: 1000}
		if err := f.Validate(); err == nil {
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
	t.Run("unknown attr op rejected", func(t *testing.T) {
		f := Filters{StartMs: 1, EndMs: 2, Attributes: []AttrFilter{{Key: "k", Op: "like", Value: "v"}}}
		if err := f.Validate(); err == nil {
			t.Error("want error for unknown op")
		}
	})
	t.Run("empty attr key rejected", func(t *testing.T) {
		f := Filters{StartMs: 1, EndMs: 2, Attributes: []AttrFilter{{Key: "", Op: "eq", Value: "v"}}}
		if err := f.Validate(); err == nil {
			t.Error("want error for empty key")
		}
	})
	t.Run("comparison requires numeric value", func(t *testing.T) {
		f := Filters{StartMs: 1, EndMs: 2, Attributes: []AttrFilter{{Key: "k", Op: "gt", Value: "x"}}}
		if err := f.Validate(); err == nil {
			t.Error("want error for non-numeric comparison value")
		}
	})
	t.Run("invalid regex rejected", func(t *testing.T) {
		f := Filters{StartMs: 1, EndMs: 2, Attributes: []AttrFilter{{Key: "k", Op: "regex", Value: "("}}}
		if err := f.Validate(); err == nil {
			t.Error("want error for invalid regex")
		}
	})
}

// No filters -> only the three base bind args, no predicate fragments.
func TestBuildClauses_Base(t *testing.T) {
	c := BuildClauses(Filters{TenantID: 1, StartMs: 1000, EndMs: 2000})
	if c.Resource != "" || c.Span != "" || c.Root != "" {
		t.Errorf("base clauses should be empty, got %+v", c)
	}
	if c.HasSpanMatch() {
		t.Error("no filters should not require span match")
	}
	m := namedArgs(c.Args)
	for _, k := range []string{"tenantID", "start", "end"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing base arg %q", k)
		}
	}
	if len(m) != 3 {
		t.Errorf("got %d args, want 3 base args: %v", len(m), m)
	}
}

// Span-matchable predicates land in Span (any-span semantics); exclusions,
// trace id, and duration stay in Root (root-span semantics).
func TestBuildClauses_SpanVsRootSplit(t *testing.T) {
	c := BuildClauses(Filters{
		StartMs: 1, EndMs: 2,
		Services:        []string{"svc"},
		Operations:      []string{"GET /"},
		ExcludeServices: []string{"noise"},
		ExcludeStatuses: []string{"OK"},
		TraceID:         "t1",
		MinDurationNs:   5,
	})
	if !strings.Contains(c.Resource, "service IN @services") {
		t.Errorf("Resource missing service clause: %q", c.Resource)
	}
	if !strings.Contains(c.Span, "name IN @operations") {
		t.Errorf("Span missing operations clause: %q", c.Span)
	}
	for _, want := range []string{
		"service IN @services",
		"service NOT IN @excServices",
		"status_code_string NOT IN @excStatuses",
		"trace_id = @traceID",
		"duration_nano >= @minDur",
	} {
		if !strings.Contains(c.Root, want) {
			t.Errorf("Root missing %q: %q", want, c.Root)
		}
	}
	if !c.HasSpanMatch() {
		t.Error("span filters should require span match")
	}
}

func TestBuildClauses_HasSpanMatch(t *testing.T) {
	cases := []struct {
		name string
		f    Filters
		want bool
	}{
		{"none", Filters{StartMs: 1, EndMs: 2}, false},
		{"service", Filters{StartMs: 1, EndMs: 2, Services: []string{"a"}}, false},
		{"environment", Filters{StartMs: 1, EndMs: 2, Environments: []string{"prod"}}, true},
		{"search", Filters{StartMs: 1, EndMs: 2, Search: "x"}, true},
		{"attr", Filters{StartMs: 1, EndMs: 2, Attributes: []AttrFilter{{Key: "k", Value: "v"}}}, true},
		{"only exclude service", Filters{StartMs: 1, EndMs: 2, ExcludeServices: []string{"a"}}, false},
		{"only duration", Filters{StartMs: 1, EndMs: 2, MinDurationNs: 1}, false},
		{"only trace id", Filters{StartMs: 1, EndMs: 2, TraceID: "t"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := BuildClauses(c.f).HasSpanMatch(); got != c.want {
				t.Errorf("HasSpanMatch = %v, want %v", got, c.want)
			}
		})
	}
}

func TestBuildClauses_NoSpansResourceReference(t *testing.T) {
	c := BuildClauses(Filters{
		StartMs: 1, EndMs: 2,
		Services:        []string{"svc"},
		Environments:    []string{"prod"},
		ExcludeServices: []string{"noise"},
	})
	all := c.Resource + c.Span + c.Root
	if strings.Contains(all, "spans_resource") || strings.Contains(all, "fingerprint") {
		t.Errorf("BuildClauses emitted obsolete CTE/table references: %q", all)
	}
}

// hasError=true means "an error anywhere in the trace" (any-span); false
// keeps root-span semantics.
func TestBuildClauses_HasError(t *testing.T) {
	tru, fls := true, false
	if c := BuildClauses(Filters{StartMs: 1, EndMs: 2, HasError: &tru}); !strings.Contains(c.Span, "has_error = 1") {
		t.Errorf("HasError=true should be a span clause, got %+v", c)
	}
	if c := BuildClauses(Filters{StartMs: 1, EndMs: 2, HasError: &fls}); !strings.Contains(c.Root, "has_error = 0") {
		t.Errorf("HasError=false should be a root clause, got %+v", c)
	}
	if c := BuildClauses(Filters{StartMs: 1, EndMs: 2}); strings.Contains(c.Span+c.Root, "has_error") {
		t.Errorf("nil HasError should emit no clause, got %+v", c)
	}
}

// Name search is always case-insensitive substring regardless of searchMode.
func TestBuildClauses_Search(t *testing.T) {
	for _, mode := range []string{"", "exact", "ngram"} {
		c := BuildClauses(Filters{StartMs: 1, EndMs: 2, Search: "x", SearchMode: mode})
		if !strings.Contains(c.Span, "positionCaseInsensitive(name, @search) > 0") {
			t.Errorf("mode %q search clause wrong: %q", mode, c.Span)
		}
	}
}

func TestBuildClauses_AttributeOps(t *testing.T) {
	cases := []struct {
		name string
		attr AttrFilter
		want string
	}{
		{"eq", AttrFilter{Key: "k", Op: "", Value: "v"}, "attributes[@akey_0] = @aval_0"},
		{"eq exp", AttrFilter{Key: "k", Op: "eq", Value: "v"}, "attributes[@akey_0] = @aval_0"},
		{
			"neq", AttrFilter{Key: "k", Op: "neq", Value: "v"},
			"(NOT (attributes[@akey_0] IS NULL) AND attributes[@akey_0] != @aval_0)",
		},
		{
			"contains", AttrFilter{Key: "k", Op: "contains", Value: "v"},
			"positionCaseInsensitive(attributes[@akey_0], @aval_0) > 0",
		},
		{"regex", AttrFilter{Key: "k", Op: "regex", Value: "v.*"}, "match(attributes[@akey_0], @aval_0)"},
		{
			"gt", AttrFilter{Key: "k", Op: "gte", Value: "4"},
			"toFloat64OrNull(attributes[@akey_0]) >= @aval_0",
		},
		{"exists", AttrFilter{Key: "k", Op: "exists"}, "NOT (attributes[@akey_0] IS NULL)"},
		{"not_exists", AttrFilter{Key: "k", Op: "not_exists"}, "attributes[@akey_0] IS NULL"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cl := BuildClauses(Filters{
				StartMs: 1, EndMs: 2,
				Attributes: []AttrFilter{c.attr},
			})
			if !strings.Contains(cl.Span, c.want) {
				t.Errorf("got %q, want substring %q", cl.Span, c.want)
			}
			if m := namedArgs(cl.Args); m["akey_0"] != "k" {
				t.Errorf("attr key bind = %v, want k", m["akey_0"])
			}
		})
	}
}

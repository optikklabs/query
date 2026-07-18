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

// No filters -> only the five base bind args, no predicate fragments.
func TestBuildClauses_Base(t *testing.T) {
	rw, w, args := BuildClauses(Filters{TenantID: 1, StartMs: 1000, EndMs: 2000})
	if rw != "" || w != "" {
		t.Errorf("base clauses should be empty, got rw=%q w=%q", rw, w)
	}
	m := namedArgs(args)
	for _, k := range []string{"tenantID", "start", "end"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing base arg %q", k)
		}
	}
	if len(m) != 3 {
		t.Errorf("got %d args, want 3 base args: %v", len(m), m)
	}
}

func TestBuildClauses_ResourceVsSpanSplit(t *testing.T) {
	rw, w, args := BuildClauses(Filters{
		StartMs: 1, EndMs: 2,
		Services:   []string{"svc"},
		Operations: []string{"GET /"},
	})
	if !strings.Contains(rw, "service IN @services") {
		t.Errorf("resourceWhere missing service clause: %q", rw)
	}
	if !strings.Contains(w, "name IN @operations") {
		t.Errorf("where missing operations clause: %q", w)
	}
	m := namedArgs(args)
	if _, ok := m["services"]; !ok {
		t.Error("services arg not bound")
	}
}

func TestBuildClauses_HasError(t *testing.T) {
	tru, fls := true, false
	if _, w, _ := BuildClauses(Filters{StartMs: 1, EndMs: 2, HasError: &tru}); !strings.Contains(w, "has_error = 1") {
		t.Errorf("HasError=true should emit has_error = 1, got %q", w)
	}
	if _, w, _ := BuildClauses(Filters{StartMs: 1, EndMs: 2, HasError: &fls}); !strings.Contains(w, "has_error = 0") {
		t.Errorf("HasError=false should emit has_error = 0, got %q", w)
	}
	if _, w, _ := BuildClauses(Filters{StartMs: 1, EndMs: 2}); strings.Contains(w, "has_error") {
		t.Errorf("nil HasError should emit no clause, got %q", w)
	}
}

func TestBuildClauses_Search(t *testing.T) {
	_, exact, _ := BuildClauses(Filters{StartMs: 1, EndMs: 2, Search: "x", SearchMode: "exact"})
	if !strings.Contains(exact, "positionCaseInsensitive(name, @search)") {
		t.Errorf("exact search clause wrong: %q", exact)
	}
	_, ng, _ := BuildClauses(Filters{StartMs: 1, EndMs: 2, Search: "x", SearchMode: "ngram"})
	if !strings.Contains(ng, "hasToken(lower(name), lower(@search))") {
		t.Errorf("ngram search clause wrong: %q", ng)
	}
}

func TestBuildClauses_AttributeOps(t *testing.T) {
	cases := []struct {
		op   string
		want string
	}{
		{"", "attributes[@akey_0]::String = @aval_0"},
		{"neq", "attributes[@akey_0]::String != @aval_0"},
		{"contains", "positionCaseInsensitive(attributes[@akey_0]::String, @aval_0) > 0"},
		{"regex", "match(attributes[@akey_0]::String, @aval_0)"},
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

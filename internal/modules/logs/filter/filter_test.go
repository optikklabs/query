package filter

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func TestSupportsStats(t *testing.T) {
	compatible := Filters{
		Services:          []string{"checkout"},
		Hosts:             []string{"host-a"},
		Pods:              []string{"pod-a"},
		Containers:        []string{"container-a"},
		Environments:      []string{"production"},
		Severities:        []string{"ERROR"},
		ExcludeServices:   []string{"worker"},
		ExcludeHosts:      []string{"host-b"},
		ExcludeSeverities: []string{"DEBUG"},
	}
	if !SupportsStats(compatible) {
		t.Fatal("promoted dimensions and severity filters must use the log rollup")
	}

	for name, mutate := range map[string]func(*Filters){
		"trace id":    func(f *Filters) { f.TraceID = "trace" },
		"span id":     func(f *Filters) { f.SpanID = "span" },
		"body search": func(f *Filters) { f.Search = "timeout" },
		"attribute":   func(f *Filters) { f.Attributes = []AttrFilter{{Key: "user.id", Value: "42"}} },
	} {
		t.Run(name, func(t *testing.T) {
			f := compatible
			mutate(&f)
			if SupportsStats(f) {
				t.Fatalf("%s is not represented in logs_stats_1m", name)
			}
		})
	}
}

func TestBuildStatsClausesUsesOnlyCompleteMinutesFromRollup(t *testing.T) {
	f := Filters{
		TenantID: 7,
		StartMs:  70_000,
		EndMs:    190_000,
		Services: []string{"checkout"},
	}
	clauses, ok := BuildStatsClauses(f)
	if !ok {
		t.Fatal("expected range with a complete minute to use the rollup")
	}

	for _, want := range []string{
		"timestamp >= @rollupStart AND timestamp < @rollupEnd",
		"timestamp < @rollupStart OR timestamp >= @rollupEnd",
		"service IN @services",
	} {
		if !strings.Contains(clauses.RawPrewhere+clauses.RollupPrewhere, want) {
			t.Errorf("missing %q from stats clauses", want)
		}
	}
	if strings.Contains(clauses.RawPrewhere+clauses.RollupPrewhere, "now()") {
		t.Fatal("stats routing must not depend on wall-clock time")
	}
	if len(clauses.Args) == 0 {
		t.Fatal("stats query lost its bound arguments")
	}
}

func TestBuildStatsClausesDoesNotApproximateSubMinuteRange(t *testing.T) {
	_, ok := BuildStatsClauses(Filters{TenantID: 1, StartMs: 1_000, EndMs: 59_000})
	if ok {
		t.Fatal("sub-minute range has no complete rollup bucket")
	}
}

// Golden contract for the logs attribute-filter SQL. Every case pins the
// exact clause string and args; do not change these without a schema change.
func TestBuildAttrClauseGolden(t *testing.T) {
	key := clickhouse.Named("akey_0", "user.id")
	cases := []struct {
		name string
		af   AttrFilter
		i    int
		sql  string
		args []any
	}{
		{
			name: "empty op defaults to eq",
			af:   AttrFilter{Key: "user.id", Value: "abc"},
			sql:  ` AND (mapContains(attributes_string, @akey_0) AND attributes_string[@akey_0] = @aval_0)`,
			args: []any{key, clickhouse.Named("aval_0", "abc")},
		},
		{
			name: "eq string value",
			af:   AttrFilter{Key: "user.id", Op: "eq", Value: "abc"},
			sql:  ` AND (mapContains(attributes_string, @akey_0) AND attributes_string[@akey_0] = @aval_0)`,
			args: []any{key, clickhouse.Named("aval_0", "abc")},
		},
		{
			name: "eq numeric value also matches attributes_number",
			af:   AttrFilter{Key: "user.id", Op: "eq", Value: "42"},
			sql: ` AND ((mapContains(attributes_string, @akey_0) AND attributes_string[@akey_0] = @aval_0) OR (mapContains(attributes_number, @akey_0)` +
				` AND attributes_number[@akey_0] = @aval_0_n))`,
			args: []any{key, clickhouse.Named("aval_0", "42"), clickhouse.Named("aval_0_n", float64(42))},
		},
		{
			name: "eq value 1 is numeric, not bool",
			af:   AttrFilter{Key: "user.id", Op: "eq", Value: "1"},
			sql: ` AND ((mapContains(attributes_string, @akey_0) AND attributes_string[@akey_0] = @aval_0) OR (mapContains(attributes_number, @akey_0)` +
				` AND attributes_number[@akey_0] = @aval_0_n))`,
			args: []any{key, clickhouse.Named("aval_0", "1"), clickhouse.Named("aval_0_n", float64(1))},
		},
		{
			name: "eq bool value also matches attributes_bool",
			af:   AttrFilter{Key: "user.id", Op: "eq", Value: "true"},
			sql: ` AND ((mapContains(attributes_string, @akey_0) AND attributes_string[@akey_0] = @aval_0) OR (mapContains(attributes_bool, @akey_0)` +
				` AND attributes_bool[@akey_0] = @aval_0_b))`,
			args: []any{key, clickhouse.Named("aval_0", "true"), clickhouse.Named("aval_0_b", true)},
		},
		{
			name: "neq string requires key in attributes_string",
			af:   AttrFilter{Key: "user.id", Op: "neq", Value: "abc"},
			sql:  ` AND (mapContains(attributes_string, @akey_0) AND attributes_string[@akey_0] != @aval_0)`,
			args: []any{key, clickhouse.Named("aval_0", "abc")},
		},
		{
			name: "neq numeric value also checks attributes_number",
			af:   AttrFilter{Key: "user.id", Op: "neq", Value: "42"},
			sql: ` AND ((mapContains(attributes_string, @akey_0) AND attributes_string[@akey_0] != @aval_0)` +
				` OR (mapContains(attributes_number, @akey_0) AND attributes_number[@akey_0] != @aval_0_n))`,
			args: []any{key, clickhouse.Named("aval_0", "42"), clickhouse.Named("aval_0_n", float64(42))},
		},
		{
			name: "neq bool value also checks attributes_bool",
			af:   AttrFilter{Key: "user.id", Op: "neq", Value: "false"},
			sql: ` AND ((mapContains(attributes_string, @akey_0) AND attributes_string[@akey_0] != @aval_0)` +
				` OR (mapContains(attributes_bool, @akey_0) AND attributes_bool[@akey_0] != @aval_0_b))`,
			args: []any{key, clickhouse.Named("aval_0", "false"), clickhouse.Named("aval_0_b", false)},
		},
		{
			name: "contains",
			af:   AttrFilter{Key: "user.id", Op: "contains", Value: "ab"},
			sql:  ` AND positionCaseInsensitive(if(mapContains(attributes_string, @akey_0), attributes_string[@akey_0], NULL), @aval_0) > 0`,
			args: []any{key, clickhouse.Named("aval_0", "ab")},
		},
		{
			name: "regex",
			af:   AttrFilter{Key: "user.id", Op: "regex", Value: "^ab.*"},
			sql:  ` AND match(if(mapContains(attributes_string, @akey_0), attributes_string[@akey_0], NULL), @aval_0)`,
			args: []any{key, clickhouse.Named("aval_0", "^ab.*")},
		},
		{
			name: "gt",
			af:   AttrFilter{Key: "user.id", Op: "gt", Value: "1.5"},
			sql: ` AND coalesce(toFloat64OrNull(attributes_string[@akey_0]),` +
				` if(mapContains(attributes_number, @akey_0), attributes_number[@akey_0], NULL)) > @aval_0`,
			args: []any{key, clickhouse.Named("aval_0", 1.5)},
		},
		{
			name: "gte",
			af:   AttrFilter{Key: "user.id", Op: "gte", Value: "1.5"},
			sql: ` AND coalesce(toFloat64OrNull(attributes_string[@akey_0]),` +
				` if(mapContains(attributes_number, @akey_0), attributes_number[@akey_0], NULL)) >= @aval_0`,
			args: []any{key, clickhouse.Named("aval_0", 1.5)},
		},
		{
			name: "lt",
			af:   AttrFilter{Key: "user.id", Op: "lt", Value: "1.5"},
			sql: ` AND coalesce(toFloat64OrNull(attributes_string[@akey_0]),` +
				` if(mapContains(attributes_number, @akey_0), attributes_number[@akey_0], NULL)) < @aval_0`,
			args: []any{key, clickhouse.Named("aval_0", 1.5)},
		},
		{
			name: "lte",
			af:   AttrFilter{Key: "user.id", Op: "lte", Value: "1.5"},
			sql: ` AND coalesce(toFloat64OrNull(attributes_string[@akey_0]),` +
				` if(mapContains(attributes_number, @akey_0), attributes_number[@akey_0], NULL)) <= @aval_0`,
			args: []any{key, clickhouse.Named("aval_0", 1.5)},
		},
		{
			name: "exists checks all three typed maps",
			af:   AttrFilter{Key: "user.id", Op: "exists"},
			sql: ` AND (mapContains(attributes_string, @akey_0)` +
				` OR mapContains(attributes_number, @akey_0)` +
				` OR mapContains(attributes_bool, @akey_0))`,
			args: []any{key},
		},
		{
			name: "not_exists checks all three typed maps",
			af:   AttrFilter{Key: "user.id", Op: "not_exists"},
			sql: ` AND NOT (mapContains(attributes_string, @akey_0)` +
				` OR mapContains(attributes_number, @akey_0)` +
				` OR mapContains(attributes_bool, @akey_0))`,
			args: []any{key},
		},
		{
			name: "unknown op emits nothing",
			af:   AttrFilter{Key: "user.id", Op: "bogus", Value: "abc"},
			sql:  "",
			args: nil,
		},
		{
			name: "bind names follow the filter index",
			af:   AttrFilter{Key: "user.id", Op: "eq", Value: "abc"},
			i:    3,
			sql:  ` AND (mapContains(attributes_string, @akey_3) AND attributes_string[@akey_3] = @aval_3)`,
			args: []any{clickhouse.Named("akey_3", "user.id"), clickhouse.Named("aval_3", "abc")},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sql, args := buildAttrClause(tc.af, tc.i)
			if sql != tc.sql {
				t.Fatalf("sql:\n got  %q\n want %q", sql, tc.sql)
			}
			if !reflect.DeepEqual(args, tc.args) {
				t.Fatalf("args:\n got  %#v\n want %#v", args, tc.args)
			}
		})
	}
}

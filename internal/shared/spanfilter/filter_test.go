package spanfilter

import (
	"reflect"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// Golden contract for the traces attribute-filter SQL. Every case pins the
// exact clause string and args; do not change these without a schema change.
func TestBuildAttrClauseGolden(t *testing.T) {
	key := clickhouse.Named("akey_0", "http.method")
	cases := []struct {
		name string
		af   AttrFilter
		i    int
		sql  string
		args []any
	}{
		{
			name: "empty op defaults to eq",
			af:   AttrFilter{Key: "http.method", Value: "GET"},
			sql:  ` AND attributes[@akey_0] = @aval_0`,
			args: []any{key, clickhouse.Named("aval_0", "GET")},
		},
		{
			name: "eq",
			af:   AttrFilter{Key: "http.method", Op: "eq", Value: "GET"},
			sql:  ` AND attributes[@akey_0] = @aval_0`,
			args: []any{key, clickhouse.Named("aval_0", "GET")},
		},
		{
			name: "neq requires attribute to exist",
			af:   AttrFilter{Key: "http.method", Op: "neq", Value: "GET"},
			sql:  ` AND (NOT (attributes[@akey_0] IS NULL) AND attributes[@akey_0] != @aval_0)`,
			args: []any{key, clickhouse.Named("aval_0", "GET")},
		},
		{
			name: "contains",
			af:   AttrFilter{Key: "http.method", Op: "contains", Value: "GE"},
			sql:  ` AND positionCaseInsensitive(attributes[@akey_0], @aval_0) > 0`,
			args: []any{key, clickhouse.Named("aval_0", "GE")},
		},
		{
			name: "regex",
			af:   AttrFilter{Key: "http.method", Op: "regex", Value: "^GE.*"},
			sql:  ` AND match(attributes[@akey_0], @aval_0)`,
			args: []any{key, clickhouse.Named("aval_0", "^GE.*")},
		},
		{
			name: "gt",
			af:   AttrFilter{Key: "http.method", Op: "gt", Value: "1.5"},
			sql:  ` AND toFloat64OrNull(attributes[@akey_0]) > @aval_0`,
			args: []any{key, clickhouse.Named("aval_0", 1.5)},
		},
		{
			name: "gte",
			af:   AttrFilter{Key: "http.method", Op: "gte", Value: "1.5"},
			sql:  ` AND toFloat64OrNull(attributes[@akey_0]) >= @aval_0`,
			args: []any{key, clickhouse.Named("aval_0", 1.5)},
		},
		{
			name: "lt",
			af:   AttrFilter{Key: "http.method", Op: "lt", Value: "1.5"},
			sql:  ` AND toFloat64OrNull(attributes[@akey_0]) < @aval_0`,
			args: []any{key, clickhouse.Named("aval_0", 1.5)},
		},
		{
			name: "lte",
			af:   AttrFilter{Key: "http.method", Op: "lte", Value: "1.5"},
			sql:  ` AND toFloat64OrNull(attributes[@akey_0]) <= @aval_0`,
			args: []any{key, clickhouse.Named("aval_0", 1.5)},
		},
		{
			name: "exists",
			af:   AttrFilter{Key: "http.method", Op: "exists"},
			sql:  ` AND NOT (attributes[@akey_0] IS NULL)`,
			args: []any{key},
		},
		{
			name: "not_exists",
			af:   AttrFilter{Key: "http.method", Op: "not_exists"},
			sql:  ` AND attributes[@akey_0] IS NULL`,
			args: []any{key},
		},
		{
			name: "unknown op emits nothing",
			af:   AttrFilter{Key: "http.method", Op: "bogus", Value: "GET"},
			sql:  "",
			args: nil,
		},
		{
			name: "bind names follow the filter index",
			af:   AttrFilter{Key: "http.method", Op: "eq", Value: "GET"},
			i:    3,
			sql:  ` AND attributes[@akey_3] = @aval_3`,
			args: []any{clickhouse.Named("akey_3", "http.method"), clickhouse.Named("aval_3", "GET")},
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

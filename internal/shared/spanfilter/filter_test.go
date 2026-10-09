package spanfilter

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// Golden contract for the traces attribute-filter SQL. Every case pins the
// exact clause string and args; do not change these without a schema change.
func TestBuildAttrClauseGolden(t *testing.T) {
	key := clickhouse.Named("akey_0", "rpc.method")
	cases := []struct {
		name string
		af   AttrFilter
		i    int
		sql  string
		args []any
	}{
		{
			name: "empty op defaults to eq",
			af:   AttrFilter{Key: "rpc.method", Value: "GET"},
			sql:  ` AND (mapContains(attributes, @akey_0) AND attributes[@akey_0] = @aval_0)`,
			args: []any{key, clickhouse.Named("aval_0", "GET")},
		},
		{
			name: "eq",
			af:   AttrFilter{Key: "rpc.method", Op: "eq", Value: "GET"},
			sql:  ` AND (mapContains(attributes, @akey_0) AND attributes[@akey_0] = @aval_0)`,
			args: []any{key, clickhouse.Named("aval_0", "GET")},
		},
		{
			name: "neq requires attribute to exist",
			af:   AttrFilter{Key: "rpc.method", Op: "neq", Value: "GET"},
			sql:  ` AND (mapContains(attributes, @akey_0) AND attributes[@akey_0] != @aval_0)`,
			args: []any{key, clickhouse.Named("aval_0", "GET")},
		},
		{
			name: "contains",
			af:   AttrFilter{Key: "rpc.method", Op: "contains", Value: "GE"},
			sql:  ` AND positionCaseInsensitive(if(mapContains(attributes, @akey_0), attributes[@akey_0], NULL), @aval_0) > 0`,
			args: []any{key, clickhouse.Named("aval_0", "GE")},
		},
		{
			name: "regex",
			af:   AttrFilter{Key: "rpc.method", Op: "regex", Value: "^GE.*"},
			sql:  ` AND match(if(mapContains(attributes, @akey_0), attributes[@akey_0], NULL), @aval_0)`,
			args: []any{key, clickhouse.Named("aval_0", "^GE.*")},
		},
		{
			name: "gt",
			af:   AttrFilter{Key: "rpc.method", Op: "gt", Value: "1.5"},
			sql:  ` AND toFloat64OrNull(attributes[@akey_0]) > @aval_0`,
			args: []any{key, clickhouse.Named("aval_0", 1.5)},
		},
		{
			name: "gte",
			af:   AttrFilter{Key: "rpc.method", Op: "gte", Value: "1.5"},
			sql:  ` AND toFloat64OrNull(attributes[@akey_0]) >= @aval_0`,
			args: []any{key, clickhouse.Named("aval_0", 1.5)},
		},
		{
			name: "lt",
			af:   AttrFilter{Key: "rpc.method", Op: "lt", Value: "1.5"},
			sql:  ` AND toFloat64OrNull(attributes[@akey_0]) < @aval_0`,
			args: []any{key, clickhouse.Named("aval_0", 1.5)},
		},
		{
			name: "lte",
			af:   AttrFilter{Key: "rpc.method", Op: "lte", Value: "1.5"},
			sql:  ` AND toFloat64OrNull(attributes[@akey_0]) <= @aval_0`,
			args: []any{key, clickhouse.Named("aval_0", 1.5)},
		},
		{
			name: "exists",
			af:   AttrFilter{Key: "rpc.method", Op: "exists"},
			sql:  ` AND mapContains(attributes, @akey_0)`,
			args: []any{key},
		},
		{
			name: "not_exists",
			af:   AttrFilter{Key: "rpc.method", Op: "not_exists"},
			sql:  ` AND NOT mapContains(attributes, @akey_0)`,
			args: []any{key},
		},
		{
			name: "unknown op emits nothing",
			af:   AttrFilter{Key: "rpc.method", Op: "bogus", Value: "GET"},
			sql:  "",
			args: nil,
		},
		{
			name: "bind names follow the filter index",
			af:   AttrFilter{Key: "rpc.method", Op: "eq", Value: "GET"},
			i:    3,
			sql:  ` AND (mapContains(attributes, @akey_3) AND attributes[@akey_3] = @aval_3)`,
			args: []any{clickhouse.Named("akey_3", "rpc.method"), clickhouse.Named("aval_3", "GET")},
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

func TestRangeRequestBindTenant(t *testing.T) {
	var req struct {
		RangeRequest
		Limit int `json:"limit"`
	}
	body := `{"startTime":1000,"endTime":2000,"services":["api"],"limit":5}`
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	if err := req.BindTenant(7); err != nil {
		t.Fatalf("BindTenant: %v", err)
	}
	if req.TenantID != 7 || req.StartMs != 1000 || req.EndMs != 2000 || req.Limit != 5 || len(req.Services) != 1 {
		t.Fatalf("bound request = %+v", req)
	}

	req.StartTime, req.EndTime = 2000, 1000
	if err := req.BindTenant(7); err == nil {
		t.Fatal("inverted range accepted")
	}
}

package repository

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/services/redfleet/filter"
)

// Renders the per-endpoint RED query with its named parameters inlined, so the
// exact statement the server sends can be pasted into clickhouse-client.
//
//	SQL_TENANT=1 SQL_SERVICE=frontend-proxy SQL_HOURS=24 \
//	    go test ./internal/modules/services/redfleet/repository -run RenderREDByEndpointSQL -v
func TestRenderREDByEndpointSQL(t *testing.T) {
	tenant := envInt64(t, "SQL_TENANT", 1)
	service := envStr("SQL_SERVICE", "frontend-proxy")
	hours := envInt64(t, "SQL_HOURS", 24)

	end := time.Now().UnixMilli()
	start := end - hours*int64(time.Hour/time.Millisecond)

	f := filter.Filters{TenantID: tenant, StartMs: start, EndMs: end}
	if service != "" {
		f.Services = []string{service}
	}

	query, args := buildREDByEndpointQuery(f)
	t.Log("\n" + inlineNamedArgs(query, args) + "\nFORMAT PrettyCompact")
}

// The row cap must cover every (endpoint, bucket) pair, or the query drops the
// tail of the window with no error anywhere.
func TestREDByEndpointRowCapCoversEveryBucket(t *testing.T) {
	for _, hours := range []int64{1, 6, 24, 24 * 7, 24 * 90, 24 * 400} {
		end := time.Now().UnixMilli()
		start := end - hours*int64(time.Hour/time.Millisecond)
		f := filter.Filters{TenantID: 1, StartMs: start, EndMs: end, Services: []string{"svc"}}

		_, args := buildREDByEndpointQuery(f)
		rowLimit, ok := namedArg(args, "rowLimit")
		if !ok {
			t.Fatalf("%dh: no rowLimit arg", hours)
		}

		grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
		buckets := (f.EndMs-f.StartMs)/grain.Milliseconds() + 1
		want := maxEndpointCardinality * buckets
		if got := rowLimit.(int64); got < want {
			t.Errorf("%dh: rowLimit %d < %d endpoint-buckets — series would truncate",
				hours, got, want)
		}
	}
}

func namedArg(args []any, name string) (any, bool) {
	for _, a := range args {
		if nv, ok := a.(driver.NamedValue); ok && nv.Name == name {
			return nv.Value, true
		}
	}
	return nil, false
}

func inlineNamedArgs(query string, args []any) string {
	named := make([]driver.NamedValue, 0, len(args))
	for _, a := range args {
		if nv, ok := a.(driver.NamedValue); ok {
			named = append(named, nv)
		}
	}
	// "@end" is a prefix of "@endpointLimit"; substituting short names first
	// would corrupt the longer ones.
	sort.SliceStable(named, func(i, j int) bool {
		return len(named[i].Name) > len(named[j].Name)
	})
	for _, nv := range named {
		query = strings.ReplaceAll(query, "@"+nv.Name, literal(nv.Value))
	}
	return query
}

func literal(v any) string {
	switch value := v.(type) {
	case string:
		return "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
	case time.Time:
		return fmt.Sprintf("toDateTime64(%d.%03d, 3)", value.Unix(), value.Nanosecond()/1e6)
	case []string:
		quoted := make([]string, len(value))
		for i, s := range value {
			quoted[i] = literal(s)
		}
		return "(" + strings.Join(quoted, ", ") + ")"
	default:
		return fmt.Sprintf("%v", value)
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt64(t *testing.T, key string, fallback int64) int64 {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var out int64
	if _, err := fmt.Sscanf(v, "%d", &out); err != nil {
		t.Fatalf("%s: %v", key, err)
	}
	return out
}

package filter

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

func testFilters() Filters {
	return Filters{TenantID: 1, MetricName: "system.cpu.utilization", StartMs: 0, EndMs: 3_600_000}
}

// The rollup carries dimensions, so nothing may reach back to metrics_series.
func TestBuildSelectionEmitsNoSeriesJoin(t *testing.T) {
	f := testFilters()
	f.GroupBy = []string{"host", "state"}
	f.Tags = []TagFilter{{Key: "service", Operator: "=", Values: []string{"api"}}}

	fromTable, where, selectCols, groupByCols, _ := BuildSelection(f)
	joined := fromTable + where + selectCols + groupByCols
	for _, banned := range []string{"metrics_series", "fingerprint", "JOIN", "WITH"} {
		if strings.Contains(joined, banned) {
			t.Errorf("BuildSelection still emits %q:\n%s", banned, joined)
		}
	}
}

// Resource keys map to real columns; anything else must stay a map lookup, or
// grouping by an arbitrary metric label silently stops working.
func TestBuildSelectionGroupByColumns(t *testing.T) {
	for key, want := range map[string]string{
		"host":          "host",
		"service.name":  "service",
		"k8s.pod.name":  "pod",
		"cloud.region":  "cloud_region",
		"k8s_namespace": "k8s_namespace",
		"state":         "attributes['state']",
		"custom.label":  "attributes['custom.label']",
	} {
		f := testFilters()
		f.GroupBy = []string{key}
		_, _, _, groupByCols, _ := BuildSelection(f)
		if !strings.Contains(groupByCols, want) {
			t.Errorf("group by %q: want %q in %q", key, want, groupByCols)
		}
	}
}

func TestBuildSelectionNoGroupByOrTagsIsBare(t *testing.T) {
	_, where, _, groupByCols, _ := BuildSelection(testFilters())
	if where != "" {
		t.Errorf("unfiltered selection should add no WHERE, got %q", where)
	}
	if groupByCols != "bucket_at" {
		t.Errorf("want bucket_at only, got %q", groupByCols)
	}
}

func TestBuildSelectionUsesSharedTagClauses(t *testing.T) {
	f := testFilters()
	f.Tags = []TagFilter{
		{Key: "service", Operator: "=", Values: []string{"api"}},
		{Key: "service.name", Operator: "IN", Values: []string{"worker"}},
		{Key: "service", Operator: "!=", Values: []string{"admin"}},
		{Key: "state", Operator: "=", Values: []string{"busy"}},
	}

	_, where, _, _, args := BuildSelection(f)
	for _, want := range []string{
		"service IN @mr0",
		"service NOT IN @xmr0",
		"mapContains(attributes, 'state')",
		"attributes['state'] = @mf0",
	} {
		if !strings.Contains(where, want) {
			t.Errorf("where clause missing %q: %s", want, where)
		}
	}

	wantArgs := []driver.NamedValue{
		{Name: "mf0", Value: "busy"},
		{Name: "mr0", Value: []string{"api", "worker"}},
		{Name: "xmr0", Value: []string{"admin"}},
	}
	if len(args) != len(wantArgs) {
		t.Fatalf("got %d args, want %d", len(args), len(wantArgs))
	}
	for i, want := range wantArgs {
		got, ok := args[i].(driver.NamedValue)
		if !ok || got.Name != want.Name || !reflect.DeepEqual(got.Value, want.Value) {
			t.Errorf("arg %d = %#v, want %#v", i, args[i], want)
		}
	}
}

func TestBucketDurationSeconds(t *testing.T) {
	now := time.Now().UnixMilli()
	for _, tc := range []struct {
		name  string
		start int64
		end   int64
		step  string
		want  int64
	}{
		{name: "explicit", start: 1, end: 1 + 24*3_600_000, step: "15m", want: 900},
		{name: "short automatic range", start: now, end: now + 2*3_600_000, want: 60},
		{name: "daily automatic range", start: now, end: now + 30*24*3_600_000, want: 86_400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := BucketDurationSeconds(tc.start, tc.end, tc.step); got != tc.want {
				t.Fatalf("BucketDurationSeconds() = %d, want %d", got, tc.want)
			}
		})
	}
}

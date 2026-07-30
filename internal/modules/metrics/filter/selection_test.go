package filter

import (
	"strings"
	"testing"
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

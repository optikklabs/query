package filter

import "strings"

import "testing"

// Day-window base filters route to the 5m grain on the 5m rollup tier.
func baseFilters() Filters {
	return Filters{TenantID: 1, StartMs: 1_000_000, EndMs: 1_000_000 + 24*3_600_000, MetricName: "m"}
}

func TestBuildSelectionShortWindowUsesMinuteRollup(t *testing.T) {
	f := baseFilters()
	f.EndMs = f.StartMs + 2*3_600_000
	from, _, _, _, _, _ := BuildSelection(f)
	if from != "optikk.metrics_1m" {
		t.Fatalf("from = %q, want optikk.metrics_1m", from)
	}
}

func TestBuildSelectionNoFilter(t *testing.T) {
	from, cte, joins, _, groupBy, _ := BuildSelection(baseFilters())
	if from != "optikk.metrics_5m" {
		t.Fatalf("from = %q", from)
	}
	if cte != "" || joins != "" {
		t.Fatalf("expected no CTE/join, got cte=%q joins=%q", cte, joins)
	}
	if groupBy != "bucket_at" {
		t.Fatalf("groupBy = %q", groupBy)
	}
}

func TestBuildSelectionAttrFilterUsesSeries(t *testing.T) {
	f := baseFilters()
	f.Tags = []TagFilter{{Key: "db.name", Operator: "=", Values: []string{"orders"}}}
	from, cte, joins, _, _, args := BuildSelection(f)
	if from != "optikk.metrics_5m" {
		t.Fatalf("from = %q", from)
	}
	if !strings.Contains(cte, "optikk.metrics_series") || !strings.Contains(cte, "fps AS") {
		t.Fatalf("cte missing metrics_series fps: %q", cte)
	}
	if !strings.Contains(cte, "attributes.`db.name`::String = @mf0") {
		t.Fatalf("cte missing attr clause: %q", cte)
	}
	if !strings.Contains(joins, "INNER JOIN fps ON m.fingerprint = fps.fingerprint") {
		t.Fatalf("joins = %q", joins)
	}
	if len(args) == 0 {
		t.Fatal("expected bind args for attr filter")
	}
}

func TestBuildSelectionGroupByCarriesLabels(t *testing.T) {
	f := baseFilters()
	f.GroupBy = []string{"service", "db.name"}
	_, cte, _, selectCols, groupBy, _ := BuildSelection(f)
	if !strings.Contains(cte, "service AS g_service") {
		t.Fatalf("cte missing resource group: %q", cte)
	}
	if !strings.Contains(cte, "attributes.`db.name`::String AS g_db.name") {
		t.Fatalf("cte missing attr group: %q", cte)
	}
	for _, want := range []string{"`group_service`", "`group_db.name`"} {
		if !strings.Contains(selectCols, want) || !strings.Contains(groupBy, want) {
			t.Fatalf("missing %s in select=%q group=%q", want, selectCols, groupBy)
		}
	}
}

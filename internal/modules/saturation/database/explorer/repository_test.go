package explorer

import (
	"strings"
	"testing"
)

func TestActiveConnectionsQueryUsesLatestValuePerSeries(t *testing.T) {
	query := activeConnectionsQuery("optikk.metrics_5m")
	for _, want := range []string{
		"FROM optikk.metrics_5m",
		"argMaxMerge(m.val_last)",
		"GROUP BY db_system, fingerprint",
		"sum(latest_value)",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("query missing %q:\n%s", want, query)
		}
	}
	if strings.Contains(query, "sum(val_sum) / sum(val_count)") {
		t.Errorf("query still averages connection gauges:\n%s", query)
	}
}

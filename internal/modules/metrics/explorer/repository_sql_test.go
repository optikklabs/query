package explorer

import (
	"strings"
	"testing"
)

func TestCumulativeRollupSQLKeepsThreeNamedStages(t *testing.T) {
	sql := cumulativeRollupSQL(
		"optikk.metrics",
		" AND service IN @mr0",
		"toStartOfMinute(timestamp) AS bucket_at, [toString(service)] AS group_values",
		"bucket_at, service",
		true,
	)

	for _, want := range []string{
		"WITH\n\t\tper_series AS (",
		"max(value) AS cval",
		"GROUP BY fingerprint, sample_at, bucket_at, service",
		"increases AS (",
		"PARTITION BY fingerprint",
		"ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW",
		"if(cval < lagInFrame(cval) OVER w, cval",
		"FROM increases",
		"sum(increase) AS val_sum",
		"GROUP BY bucket_at, group_values",
		"HAVING bucket_at >= @displayStart",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("cumulative SQL missing %q:\n%s", want, sql)
		}
	}
}

func TestCumulativeRollupSQLWithoutGrouping(t *testing.T) {
	sql := cumulativeRollupSQL(
		"optikk.metrics",
		"",
		"toStartOfMinute(timestamp) AS bucket_at",
		"bucket_at",
		false,
	)
	if strings.Contains(sql, "group_values") {
		t.Fatalf("ungrouped cumulative SQL contains group_values:\n%s", sql)
	}
	if !strings.Contains(sql, "GROUP BY bucket_at") {
		t.Fatalf("ungrouped cumulative SQL does not aggregate by bucket:\n%s", sql)
	}
}

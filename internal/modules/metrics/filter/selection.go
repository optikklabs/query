package filter

import (
	"strconv"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// BuildSelection returns the rollup table, a metrics_series CTE, joins, and
// column selections. Resource and attribute filters both resolve to a
// fingerprint set in the fps CTE; the rollup is INNER JOINed on fingerprint and
// the query groups by the label value carried out of the CTE.
func BuildSelection(f Filters) (fromTable, cte, joins, selectCols, groupByCols string, args []any) {
	resourceWhere, attrWhere, args := BuildClauses(f)

	fromTable = rollupTable(f.StartMs, f.EndMs, f.Step)
	selectCols = bucketGrainSQL(f.StartMs, f.EndMs, f.Step) + " AS bucket_at"
	groupByCols = "bucket_at"

	if resourceWhere == "" && attrWhere == "" && len(f.GroupBy) == 0 {
		return fromTable, "", "", selectCols, groupByCols, args
	}

	fpsSel := "fingerprint"
	for _, key := range f.GroupBy {
		fpsSel += ", any(" + seriesColumn(key) + ") AS g_" + SanitizeKey(key)
	}
	where := resourceWhere + attrWhere
	if where != "" {
		where = "\n		    WHERE 1=1" + where
	}
	cte = `WITH fps AS (
		    SELECT ` + fpsSel + `
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND metric_name = @metricName AND timestamp BETWEEN @start AND @end` + where + `
		    GROUP BY fingerprint
		)
`
	joins = " INNER JOIN fps ON m.fingerprint = fps.fingerprint"
	for _, key := range f.GroupBy {
		alias := "`group_" + SanitizeKey(key) + "`"
		selectCols += ", fps.g_" + SanitizeKey(key) + " AS " + alias
		groupByCols += ", " + alias
	}
	return fromTable, cte, joins, selectCols, groupByCols, args
}

// rollupTable picks the read table for the effective grain: the 1m rollup for
// sub-hour grains (1m/5m/15m), the 1h rollup for hourly+ grains.
func rollupTable(startMs, endMs int64, step string) string {
	switch g := BucketDurationSeconds(startMs, endMs, step); {
	case g < 3600:
		return "optikk.metrics_1m"
	default:
		return "optikk.metrics_1h"
	}
}

// seriesColumn returns the metrics_series column expression for a group key:
// a resource column for canonical resource keys, else a JSON attribute path.
func seriesColumn(key string) string {
	if canonical := Canonical(key); canonical != "" {
		return ResourceColumn(canonical)
	}
	return "attributes.`" + SanitizeKey(key) + "`::String"
}

// BuildTagValueArms returns one SELECT arm per key and the per-key named bind
// args. Resource and attribute arms both read metrics_series scoped by
// metric_name — fingerprint is the only key, so no pre-filter CTE is needed.
func BuildTagValueArms(keys []string) (arms []string, args []any) {
	for i, key := range keys {
		label := "k" + strconv.Itoa(i)
		args = append(args, clickhouse.Named(label, key))
		col := seriesColumn(key)
		if col == "" {
			continue
		}
		arms = append(arms, `
			SELECT @`+label+` AS tag_key, `+col+` AS tag_value, count() AS c
			FROM optikk.metrics_series
			PREWHERE team_id     = @teamID
			     AND timestamp   BETWEEN @start AND @end
			     AND metric_name = @metricName
			WHERE `+col+` != ''
			GROUP BY tag_value`)
	}
	return arms, args
}

// BucketDurationSeconds returns the bucket duration for a step or grain.
func BucketDurationSeconds(startMs, endMs int64, step string) int64 {
	switch step {
	case "1m":
		// Finest grain: served from the 1m rollup.
		return 60
	case "5m":
		return 300
	case "15m":
		return 900
	case "1h":
		return 3600
	case "1d":
		return 86400
	default:
		h := (endMs - startMs) / 3_600_000
		switch {
		case h <= 2:
			return 60
		case h <= 24:
			return 300
		case h <= 168:
			return 3600
		default:
			return 86400
		}
	}
}

// bucketGrainSQL returns toStartOf* fragment matching BucketDurationSeconds.
func bucketGrainSQL(startMs, endMs int64, step string) string {
	switch BucketDurationSeconds(startMs, endMs, step) {
	case 60:
		return "toStartOfMinute(timestamp)"
	case 900:
		return "toStartOfFifteenMinutes(timestamp)"
	case 3600:
		return "toStartOfHour(timestamp)"
	case 86400:
		return "toStartOfDay(timestamp)"
	default: // 300
		return "toStartOfFiveMinutes(timestamp)"
	}
}

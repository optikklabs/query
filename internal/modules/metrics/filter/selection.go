package filter

import (
	"strconv"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/infra/timebucket"
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
	fpsGrp := "fingerprint"
	for _, key := range f.GroupBy {
		alias := "g_" + SanitizeKey(key)
		fpsSel += ", " + seriesColumn(key) + " AS " + alias
		fpsGrp += ", " + alias
	}
	where := resourceWhere + attrWhere
	if where != "" {
		where = "\n		    WHERE 1=1" + where
	}
	cte = `WITH fps AS (
		    SELECT ` + fpsSel + `
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND metric_name = @metricName AND timestamp BETWEEN @start AND @end` + where + `
		    GROUP BY ` + fpsGrp + `
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

func rollupTable(startMs, endMs int64, step string) string {
	return timebucket.RollupTableForGrain(BucketDurationSeconds(startMs, endMs, step))
}

func seriesColumn(key string) string {
	if canonical := Canonical(key); canonical != "" {
		return ResourceColumn(canonical)
	}
	return "attributes.`" + SanitizeKey(key) + "`::String"
}

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

func BucketDurationSeconds(startMs, endMs int64, step string) int64 {
	switch step {
	case "1m":

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

func bucketGrainSQL(startMs, endMs int64, step string) string {
	return timebucket.GrainSQL(BucketDurationSeconds(startMs, endMs, step))
}

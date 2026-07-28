package filter

import (
	"strconv"
	"strings"

	"github.com/optikklabs/query/internal/infra/timebucket"
)

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
	for i, key := range f.GroupBy {
		alias := "g" + strconv.Itoa(i)
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
		    PREWHERE tenant_id = @tenantID AND metric_name = @metricName AND timestamp BETWEEN @start AND @end` + where + `
		    GROUP BY ` + fpsGrp + `
		)
`
	joins = " INNER JOIN fps ON m.fingerprint = fps.fingerprint"
	groupValues := make([]string, 0, len(f.GroupBy))
	for i := range f.GroupBy {
		column := "fps.g" + strconv.Itoa(i)
		groupByCols += ", " + column
		groupValues = append(groupValues, "toString("+column+")")
	}
	if len(groupValues) > 0 {
		selectCols += ", [" + strings.Join(groupValues, ", ") + "] AS group_values"
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
	return AttrColumn(key)
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

package filter

import (
	"strings"

	"github.com/optikklabs/query/internal/infra/timebucket"
)

func BuildSelection(f Filters) (fromTable, where, selectCols, groupByCols string, args []any) {
	resourceWhere, attrWhere, args := BuildClauses(f)

	fromTable = rollupTable(f.StartMs, f.EndMs, f.Step)
	selectCols = bucketGrainSQL(f.StartMs, f.EndMs, f.Step) + " AS bucket_at"
	groupByCols = "bucket_at"
	where = resourceWhere + attrWhere

	groupValues := make([]string, 0, len(f.GroupBy))
	for _, key := range f.GroupBy {
		column := seriesColumn(key)
		groupByCols += ", " + column
		groupValues = append(groupValues, "toString("+column+")")
	}
	if len(groupValues) > 0 {
		selectCols += ", [" + strings.Join(groupValues, ", ") + "] AS group_values"
	}
	return fromTable, where, selectCols, groupByCols, args
}

func rollupTable(startMs, endMs int64, step string) string {
	return timebucket.RollupTableForGrain(BucketDurationSeconds(startMs, endMs, step))
}

func seriesColumn(key string) string {
	if canonical := Canonical(key); canonical != "" {
		return canonical
	}
	return AttrColumn(key)
}

func BucketDurationSeconds(startMs, endMs int64, step string) int64 {
	var grain int64
	switch step {
	case "1m":
		grain = 60
	case "5m":
		grain = 300
	case "15m":
		grain = 900
	case "1h":
		grain = 3600
	case "1d":
		grain = 86400
	default:
		h := (endMs - startMs) / 3_600_000
		switch {
		case h <= 2:
			grain = 60
		case h <= 24:
			grain = 300
		case h <= 168:
			grain = 3600
		default:
			grain = 86400
		}
	}
	return grain
}

func bucketGrainSQL(startMs, endMs int64, step string) string {
	return timebucket.GrainSQL(BucketDurationSeconds(startMs, endMs, step))
}

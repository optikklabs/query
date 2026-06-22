package explorer

import (
	"context"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/metrics/filter"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListMetricNames(ctx context.Context, teamID, startMs, endMs int64, search string) ([]metricNameDTO, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	// Active metric names come from the metrics_series metadata table directly.
	query := `
		SELECT metric_name,
		       any(metric_type) AS metric_type,
		       any(unit)        AS unit,
		       any(description) AS description
		FROM optikk.metrics_series
		PREWHERE team_id = @teamID
		     AND timestamp BETWEEN @start AND @end
		WHERE metric_name ILIKE @search
		GROUP BY metric_name
		ORDER BY metric_name
		LIMIT 100`
	args := []any{
		clickhouse.Named("teamID", uint32(teamID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("search", "%"+search+"%"),
	}
	var rows []metricNameDTO
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "metrics.ListMetricNames", &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) ListAttributeTagKeys(ctx context.Context, teamID, startMs, endMs int64, metricName string) ([]tagKeyDTO, error) {
	// Reads distinct attribute keys from metrics_series (metric_name leads PK).
	const dynamicQuery = `
		SELECT DISTINCT arrayJoin(mapKeys(JSONAllPathsWithTypes(attributes))) AS tag_key
		FROM optikk.metrics_series
		PREWHERE team_id     = @teamID
		     AND timestamp   BETWEEN @start AND @end
		     AND metric_name = @metricName
		ORDER BY tag_key
		LIMIT 200
		SETTINGS use_query_cache = 0`

	dynamicArgs := []any{
		clickhouse.Named("teamID", uint32(teamID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("metricName", metricName),
	}

	var rows []tagKeyDTO
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "metrics.ListTagKeys.GetDynamicKeys", &rows, dynamicQuery, dynamicArgs...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) ListResourceTagValues(ctx context.Context, teamID, startMs, endMs int64, metricName, canonical string) ([]tagValueDTO, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	col := filter.ResourceColumn(canonical)
	if col == "" {
		return nil, nil
	}
	query := `
		SELECT ` + col + ` AS tag_value,
		       count()    AS count
		FROM optikk.metrics_series
		PREWHERE team_id     = @teamID
		     AND timestamp   BETWEEN @start AND @end
		     AND metric_name = @metricName
		WHERE ` + col + ` != ''
		GROUP BY tag_value
		ORDER BY count DESC
		LIMIT 100`
	args := []any{
		clickhouse.Named("teamID", uint32(teamID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("metricName", metricName),
	}
	var rows []tagValueDTO
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "metrics.ListResourceTagValues", &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) ListAttributeTagValues(ctx context.Context, teamID, startMs, endMs int64, metricName, tagKey string) ([]tagValueDTO, error) {
	col := filter.AttrColumn(tagKey)
	query := `
		SELECT ` + col + ` AS tag_value,
		       count()      AS count
		FROM optikk.metrics_series
		PREWHERE team_id     = @teamID
		     AND timestamp   BETWEEN @start AND @end
		     AND metric_name = @metricName
		WHERE ` + col + ` != ''
		GROUP BY tag_value
		ORDER BY count DESC
		LIMIT 100`

	args := []any{
		clickhouse.Named("teamID", uint32(teamID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("metricName", metricName),
	}
	var rows []tagValueDTO
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "metrics.ListAttributeTagValues", &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

// ListTagValuesForKeys returns distinct values and counts for tag keys.
func (r *Repository) ListTagValuesForKeys(ctx context.Context, teamID, startMs, endMs int64, metricName string, keys []string) ([]tagKeyValueDTO, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	if len(keys) == 0 {
		return nil, nil
	}
	arms, armArgs := filter.BuildTagValueArms(keys)
	if len(arms) == 0 {
		return nil, nil
	}
	args := []any{
		clickhouse.Named("teamID", uint32(teamID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("metricName", metricName),
	}
	args = append(args, armArgs...)

	query := `
		SELECT tag_key, tag_value, sum(c) AS count
		FROM (` + strings.Join(arms, "\n\t\t\tUNION ALL") + `
		)
		GROUP BY tag_key, tag_value
		ORDER BY tag_key, count DESC
		LIMIT 100 BY tag_key`

	var rows []tagKeyValueDTO
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "metrics.ListTagValuesForKeys", &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

// IsCumulativeCounter reports whether the metric is a cumulative monotonic
// counter, which must be converted to per-series deltas at read time rather than
// summed across cumulative data points.
func (r *Repository) IsCumulativeCounter(ctx context.Context, f filter.Filters) (bool, error) {
	query := `
		SELECT any(temporality)  AS temporality,
		       any(is_monotonic) AS is_monotonic
		FROM optikk.metrics_series
		PREWHERE team_id     = @teamID
		     AND metric_name = @metricName
		     AND timestamp   BETWEEN @start AND @end`
	var rows []metricKindDTO
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "metrics.IsCumulativeCounter", &rows, query, metricArgs(f)...); err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, nil
	}
	return rows[0].Temporality == "Cumulative" && rows[0].IsMonotonic, nil
}

// QueryRollupSeries queries aggregated scalar metrics rollups for timeseries.
func (r *Repository) QueryRollupSeries(ctx context.Context, f filter.Filters) ([]timeseriesPointDTO, error) {
	if filter.BucketDurationSeconds(f.StartMs, f.EndMs, f.Step) >= 3600 {
		f.StartMs = timebucket.FloorMsToHour(f.StartMs)
	}
	fromTable, cte, joins, selectCols, groupByCols, filterArgs := filter.BuildSelection(f)

	var query string
	if f.Cumulative {
		query = cumulativeRollupQuery(cte, fromTable, joins, selectCols, groupByCols)
	} else {
		query = deltaRollupQuery(cte, fromTable, joins, selectCols, groupByCols)
	}

	args := append(metricArgs(f), filterArgs...)
	var rows []timeseriesPointDTO
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "metrics.QueryRollupSeries", &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

// deltaRollupQuery sums delta/gauge rollup aggregates directly per bucket.
func deltaRollupQuery(cte, fromTable, joins, selectCols, groupByCols string) string {
	return cte + `
		SELECT ` + selectCols + `,
		       sum(val_sum)   AS val_sum,
		       sum(val_count) AS val_count,
		       min(val_min)   AS val_min,
		       max(val_max)   AS val_max
		FROM ` + fromTable + ` AS m` + joins + `
		PREWHERE m.team_id     = @teamID
		     AND m.metric_name = @metricName
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY ` + groupByCols + `
		ORDER BY bucket_at ASC
		LIMIT 10000
		SETTINGS max_execution_time = 30`
}

// cumulativeRollupQuery converts a cumulative monotonic counter to per-bucket
// increase: take each series' last cumulative value per bucket, diff it against
// the prior bucket within the series (window-partitioned, so block-safe), guard
// the first bucket and counter resets, then total across series per bucket.
func cumulativeRollupQuery(cte, fromTable, joins, selectCols, groupByCols string) string {
	perSeries := cte + `
		SELECT m.fingerprint AS fingerprint, ` + selectCols + `,
		       anyLast(m.val_last) AS cval
		FROM ` + fromTable + ` AS m` + joins + `
		PREWHERE m.team_id     = @teamID
		     AND m.metric_name = @metricName
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY m.fingerprint, ` + groupByCols + ``

	increase := `
		SELECT ` + groupByCols + `,
		       if(row_number() OVER w = 1, 0,
		          if(cval < lagInFrame(cval) OVER w, cval,
		             cval - lagInFrame(cval) OVER w)) AS increase
		FROM (` + perSeries + `)
		WINDOW w AS (PARTITION BY fingerprint ORDER BY bucket_at)`

	// val_count/min/max are unused for cumulative reads (applyAggregation routes
	// every aggregation to the increase in val_sum); keep types matching the DTO.
	return `
		SELECT ` + groupByCols + `,
		       sum(increase) AS val_sum,
		       toUInt64(0)   AS val_count,
		       toFloat64(0)  AS val_min,
		       toFloat64(0)  AS val_max
		FROM (` + increase + `)
		GROUP BY ` + groupByCols + `
		ORDER BY bucket_at ASC
		LIMIT 10000
		SETTINGS max_execution_time = 30`
}

func metricArgs(f filter.Filters) []any {
	return []any{
		clickhouse.Named("teamID", uint32(f.TeamID)),
		clickhouse.Named("metricName", f.MetricName),
		clickhouse.Named("start", time.UnixMilli(f.StartMs)),
		clickhouse.Named("end", time.UnixMilli(f.EndMs)),
	}
}

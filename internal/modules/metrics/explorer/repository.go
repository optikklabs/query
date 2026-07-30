package explorer

import (
	"context"
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

func (r *Repository) ListMetricNames(ctx context.Context, tenantID, startMs, endMs int64, search string) ([]metricNameDTO, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)

	query := `
		SELECT metric_name,
		       any(metric_type) AS metric_type,
		       any(unit)        AS unit,
		       any(description) AS description
		FROM optikk.metrics_series
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		WHERE metric_name ILIKE @search
		GROUP BY metric_name
		ORDER BY metric_name
		LIMIT 100`
	args := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
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

func (r *Repository) ListResourceTagValues(ctx context.Context, tenantID, startMs, endMs int64, metricName, canonical string) ([]tagValueDTO, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	col := filter.ResourceColumn(canonical)
	if col == "" {
		return nil, nil
	}
	query := `
		SELECT ` + col + ` AS tag_value,
		       count()    AS count
		FROM optikk.metrics_series
		PREWHERE tenant_id     = @tenantID
		     AND timestamp   BETWEEN @start AND @end
		     AND metric_name = @metricName
		WHERE ` + col + ` != ''
		GROUP BY tag_value
		ORDER BY count DESC
		LIMIT 100`
	args := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
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

func (r *Repository) ListAttributeTagValues(ctx context.Context, tenantID, startMs, endMs int64, metricName, tagKey string) ([]tagValueDTO, error) {
	col := filter.AttrColumn(tagKey)
	query := `
		SELECT ` + col + ` AS tag_value,
		       count()      AS count
		FROM optikk.metrics_series
		PREWHERE tenant_id     = @tenantID
		     AND timestamp   BETWEEN @start AND @end
		     AND metric_name = @metricName
		WHERE ` + col + ` != ''
		GROUP BY tag_value
		ORDER BY count DESC
		LIMIT 100`

	args := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
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

func (r *Repository) ListTagValuesAllKeys(ctx context.Context, tenantID, startMs, endMs int64, metricName string) ([]tagKeyValueDTO, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)

	// One scan for every key: dynamic attribute entries and the four static
	// resource columns folded into the same ARRAY JOIN. The final LIMIT caps
	// the response at ~200 keys, matching the old per-key-arm limit.
	query := `
		SELECT kv.1    AS tag_key,
		       kv.2    AS tag_value,
		       count() AS count
		FROM optikk.metrics_series
		ARRAY JOIN arrayConcat(
		    CAST(attributes, 'Array(Tuple(String, String))'),
		    [('service', toString(service)), ('host', toString(host)),
		     ('environment', toString(environment)), ('k8s_namespace', toString(k8s_namespace))]
		) AS kv
		PREWHERE tenant_id     = @tenantID
		     AND timestamp   BETWEEN @start AND @end
		     AND metric_name = @metricName
		WHERE kv.2 != ''
		GROUP BY tag_key, tag_value
		ORDER BY tag_key, count DESC
		LIMIT 100 BY tag_key
		LIMIT 20000`

	args := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("metricName", metricName),
	}
	var rows []tagKeyValueDTO
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "metrics.ListTagValuesAllKeys", &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) ResolveSeriesKinds(
	ctx context.Context,
	tenantID, startMs, endMs int64,
	metricNames []string,
) (map[string]metricKindDTO, error) {
	if len(metricNames) == 0 {
		return map[string]metricKindDTO{}, nil
	}
	query := `
		SELECT metric_name,
		       any(temporality)  AS temporality,
		       any(is_monotonic) AS is_monotonic,
		       any(metric_type)  AS metric_type
		FROM optikk.metrics_series
		PREWHERE tenant_id     = @tenantID
		     AND metric_name IN @metricNames
		     AND timestamp   BETWEEN @start AND @end
		GROUP BY metric_name`
	args := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("metricNames", metricNames),
	}
	var rows []metricKindDTO
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "metrics.ResolveSeriesKinds", &rows, query, args...); err != nil {
		return nil, err
	}
	kinds := make(map[string]metricKindDTO, len(rows))
	for _, row := range rows {
		kinds[row.MetricName] = row
	}
	return kinds, nil
}

func (r *Repository) QueryRollupSeries(ctx context.Context, f filter.Filters) ([]timeseriesPointDTO, error) {
	bucketSec := filter.BucketDurationSeconds(f.StartMs, f.EndMs, f.Step)
	f.StartMs = timebucket.FloorMsToBucket(f.StartMs, bucketSec)
	fromTable, where, selectCols, groupByCols, filterArgs := filter.BuildSelection(f)

	var query string
	switch {
	case f.Histogram && isPercentile(f.Aggregation):
		query = histogramQuantileQuery(fromTable, where, selectCols, groupByCols)
	case f.Cumulative:
		query = cumulativeRollupQuery(fromTable, where, selectCols, groupByCols)
	default:
		query = deltaRollupQuery(fromTable, where, selectCols, groupByCols)
	}

	args := append(metricArgs(f), filterArgs...)
	var rows []timeseriesPointDTO
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "metrics.QueryRollupSeries", &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func deltaRollupQuery(fromTable, where, selectCols, groupByCols string) string {
	return `
		SELECT ` + selectCols + `,
		       sum(val_sum)    AS val_sum,
		       sum(val_count)  AS val_count,
		       min(val_min)    AS val_min,
		       max(val_max)    AS val_max,
		       sum(hist_sum)   AS hist_sum,
		       sum(hist_count) AS hist_count
		FROM ` + fromTable + `
		PREWHERE tenant_id     = @tenantID
		     AND metric_name = @metricName
		     AND timestamp   BETWEEN @start AND @end` + where + `
		GROUP BY ` + groupByCols + `
		ORDER BY bucket_at ASC
		LIMIT 10000
		SETTINGS max_execution_time = 30`
}

func histogramQuantileQuery(fromTable, where, selectCols, groupByCols string) string {
	return `
		SELECT ` + selectCols + `,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(latency_state) AS quantiles
		FROM ` + fromTable + `
		PREWHERE tenant_id     = @tenantID
		     AND metric_name = @metricName
		     AND timestamp   BETWEEN @start AND @end` + where + `
		GROUP BY ` + groupByCols + `
		ORDER BY bucket_at ASC
		LIMIT 10000
		SETTINGS max_execution_time = 30`
}

func cumulativeRollupQuery(fromTable, where, selectCols, groupByCols string) string {
	perSeries := `
		SELECT fingerprint, ` + selectCols + `,
		       argMaxMerge(val_last) AS cval
		FROM ` + fromTable + `
		PREWHERE tenant_id     = @tenantID
		     AND metric_name = @metricName
		     AND timestamp   BETWEEN @start AND @end` + where + `
		GROUP BY fingerprint, ` + groupByCols + ``

	increase := `
		SELECT ` + groupByCols + `,
		       if(row_number() OVER w = 1, 0,
		          if(cval < lagInFrame(cval) OVER w, cval,
		             cval - lagInFrame(cval) OVER w)) AS increase
		FROM (` + perSeries + `)
		WINDOW w AS (PARTITION BY fingerprint ORDER BY bucket_at)`

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
		clickhouse.Named("tenantID", uint32(f.TenantID)),
		clickhouse.Named("metricName", f.MetricName),
		clickhouse.Named("start", time.UnixMilli(f.StartMs)),
		clickhouse.Named("end", time.UnixMilli(f.EndMs)),
	}
}

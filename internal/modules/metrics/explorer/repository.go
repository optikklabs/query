package explorer

import (
	"context"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/metrics/filter"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListMetricNames(ctx context.Context, tenantID, startMs, endMs int64, search string) ([]metricNameDTO, error) {
	query := `
		SELECT metric_name,
		       argMax(metric_type, (timestamp, fingerprint))  AS metric_type,
		       argMax(unit, (timestamp, fingerprint))         AS unit,
		       argMax(description, (timestamp, fingerprint))  AS description,
		       argMax(temporality, (timestamp, fingerprint))  AS temporality,
		       argMax(is_monotonic, (timestamp, fingerprint)) AS is_monotonic
		FROM optikk.metrics_series
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		WHERE metric_name ILIKE @search
		GROUP BY metric_name
		ORDER BY metric_name
		LIMIT 100`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("search", filterutil.LikeSubstringPattern(search)),
	)
	var rows []metricNameDTO
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "metrics.ListMetricNames", &rows, query, args...)
	return rows, err
}

func (r *Repository) ListResourceTagValues(ctx context.Context, tenantID, startMs, endMs int64, metricName, canonical string) ([]tagValueDTO, error) {
	col := seriesResourceColumn(canonical)
	if col == "" {
		return nil, nil
	}
	query := `
		SELECT ` + col + ` AS tag_value,
		       uniqExact(fingerprint) AS count
		FROM optikk.metrics_series
		PREWHERE tenant_id     = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND metric_name = @metricName
		WHERE ` + col + ` != ''
		GROUP BY tag_value
		ORDER BY count DESC, tag_value ASC
		LIMIT 100`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("metricName", metricName),
	)
	var rows []tagValueDTO
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "metrics.ListResourceTagValues", &rows, query, args...)
	return rows, err
}

func (r *Repository) ListAttributeTagValues(ctx context.Context, tenantID, startMs, endMs int64, metricName, tagKey string) ([]tagValueDTO, error) {
	col := filter.AttrColumn(tagKey)
	query := `
		SELECT ` + col + ` AS tag_value,
		       uniqExact(fingerprint) AS count
		FROM optikk.metrics_series
		PREWHERE tenant_id     = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND metric_name = @metricName
		WHERE ` + col + ` != ''
		GROUP BY tag_value
		ORDER BY count DESC, tag_value ASC
		LIMIT 100`

	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("metricName", metricName),
	)
	var rows []tagValueDTO
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "metrics.ListAttributeTagValues", &rows, query, args...)
	return rows, err
}

func (r *Repository) ListMetricTagKeys(ctx context.Context, tenantID, startMs, endMs int64, metricName string) ([]string, error) {
	query := `
		SELECT DISTINCT arrayJoin(mapKeys(attributes)) AS tag_key
		FROM optikk.metrics_series
		PREWHERE tenant_id     = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND metric_name = @metricName
		ORDER BY tag_key
		LIMIT 200`

	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("metricName", metricName),
	)
	var rows []tagKeyDTO
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "metrics.ListMetricTagKeys", &rows, query, args...); err != nil {
		return nil, err
	}
	keys := make([]string, len(rows))
	for i, row := range rows {
		keys[i] = row.TagKey
	}
	return keys, nil
}

var seriesResourceKeys = map[string]string{
	"k8s_node": "k8s.node.name", "cloud_provider": "cloud.provider",
	"cloud_account": "cloud.account.id", "cloud_region": "cloud.region",
	"cloud_platform": "cloud.platform",
}

func seriesResourceColumn(canonical string) string {
	if key := seriesResourceKeys[canonical]; key != "" {
		return "resource_attributes['" + key + "']"
	}
	return canonical
}

func (r *Repository) ResolveMetricKinds(
	ctx context.Context,
	tenantID, startMs, endMs int64,
	metricNames []string,
) (map[string]metricNameDTO, error) {
	if len(metricNames) == 0 {
		return map[string]metricNameDTO{}, nil
	}

	// metrics_series replaces metadata within six-hour identity buckets. Widen
	// the requested range so the surviving representative still classifies data
	// near either boundary.
	query := `
		SELECT metric_name,
		       argMax(ms.temporality, (ms.timestamp, ms.fingerprint))  AS temporality,
		       argMax(ms.is_monotonic, (ms.timestamp, ms.fingerprint)) AS is_monotonic,
		       argMax(ms.metric_type, (ms.timestamp, ms.fingerprint))  AS metric_type,
		       uniqExact((ms.temporality, ms.is_monotonic, ms.metric_type)) AS variants
		FROM optikk.metrics_series AS ms
		PREWHERE ms.tenant_id     = @tenantID
		     AND ms.metric_name IN @metricNames
		     AND ms.timestamp >= @start - INTERVAL 6 HOUR
		     AND ms.timestamp < @end + INTERVAL 6 HOUR
		GROUP BY metric_name`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("metricNames", metricNames),
	)

	var rows []metricNameDTO
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "metrics.ResolveMetricKinds", &rows, query, args...); err != nil {
		return nil, err
	}

	kinds := make(map[string]metricNameDTO, len(rows))
	for _, row := range rows {
		kinds[row.MetricName] = row
	}
	return kinds, nil
}

func (r *Repository) QueryRollupSeries(ctx context.Context, f filter.Filters) ([]timeseriesPointDTO, error) {
	displayStart := f.StartMs
	if f.Cumulative {
		f.StartMs = displayStart - int64(time.Hour/time.Millisecond)
	}

	fromTable, where, selectCols, groupByCols, filterArgs := filter.BuildSelection(f)

	var sql string
	switch {
	case f.Histogram && f.Cumulative:
		sql = cumulativeHistogramSQL(where, selectCols, groupByCols, len(f.GroupBy) > 0)
	case f.Histogram && strings.HasPrefix(f.Aggregation, "p"):
		sql = histogramQuantileSQL(fromTable, where, selectCols, groupByCols)
	case f.Cumulative:
		sql = cumulativeRollupSQL("optikk.metrics", where, selectCols, groupByCols, len(f.GroupBy) > 0)
	default:
		sql = deltaRollupSQL(fromTable, where, selectCols, groupByCols)
	}

	args := append(metricArgs(f), filterArgs...)
	if f.Cumulative {
		args = append(args, chargs.Millis("displayStart", displayStart))
	}

	var rows []timeseriesPointDTO
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "metrics.QueryRollupSeries", &rows, sql, args...)
	return rows, err
}

func deltaRollupSQL(fromTable, where, selectCols, groupByCols string) string {
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
		     AND timestamp >= @start AND timestamp < @end` + where + `
		GROUP BY ` + groupByCols + `
		ORDER BY bucket_at ASC
		SETTINGS max_execution_time = 30`
}

func histogramQuantileSQL(fromTable, where, selectCols, groupByCols string) string {
	return `
		SELECT ` + selectCols + `,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(latency_state) AS quantiles
		FROM ` + fromTable + `
		PREWHERE tenant_id     = @tenantID
		     AND metric_name = @metricName
		     AND timestamp >= @start AND timestamp < @end` + where + `
		GROUP BY ` + groupByCols + `
		ORDER BY bucket_at ASC
		SETTINGS max_execution_time = 30`
}

func cumulativeRollupSQL(fromTable, where, selectCols, groupByCols string, grouped bool) string {
	resultCols := "bucket_at"
	if grouped {
		resultCols += ", group_values"
	}
	return `
		WITH
		per_series AS (
			SELECT fingerprint, timestamp AS sample_at, ` + selectCols + `,
			       max(value) AS cval
			FROM ` + fromTable + `
			PREWHERE tenant_id     = @tenantID
			     AND metric_name = @metricName
			     AND timestamp >= @start AND timestamp < @end` + where + `
			GROUP BY fingerprint, sample_at, ` + groupByCols + `
		),
		increases AS (
			SELECT ` + resultCols + `,
			       if(row_number() OVER w = 1, 0,
			          if(cval < lagInFrame(cval) OVER w, cval,
			             cval - lagInFrame(cval) OVER w)) AS increase
			FROM per_series
			WINDOW w AS (
				PARTITION BY fingerprint
				ORDER BY sample_at
				ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
			)
		)
		SELECT ` + resultCols + `,
		       sum(increase) AS val_sum,
		       toUInt64(0)   AS val_count,
		       toFloat64(0)  AS val_min,
		       toFloat64(0)  AS val_max
		FROM increases
		GROUP BY ` + resultCols + `
		HAVING bucket_at >= @displayStart
		ORDER BY bucket_at ASC
		SETTINGS max_execution_time = 30`
}

// cumulativeHistogramSQL turns each series' cumulative histogram into
// per-sample increases, as cumulativeRollupSQL does for counters: a drop in
// count or a changed bucket layout is a reset. The increases then merge
// across series like the delta rollups (see 05_metrics_rollups.sql).
func cumulativeHistogramSQL(where, selectCols, groupByCols string, grouped bool) string {
	resultCols := "bucket_at"
	if grouped {
		resultCols += ", group_values"
	}
	return `
		WITH
		per_series AS (
			SELECT fingerprint, timestamp AS sample_at, ` + selectCols + `,
			       any(hist_buckets) AS bounds,
			       any(hist_counts)  AS counts,
			       any(hist_sum)     AS hsum,
			       any(hist_count)   AS hcount
			FROM optikk.metrics
			PREWHERE tenant_id     = @tenantID
			     AND metric_name = @metricName
			     AND timestamp >= @start AND timestamp < @end` + where + `
			GROUP BY fingerprint, sample_at, ` + groupByCols + `
		),
		increases AS (
			SELECT ` + resultCols + `, bounds,
			       row_number() OVER w = 1 AS is_first,
			       hcount < lagInFrame(hcount) OVER w OR bounds != lagInFrame(bounds) OVER w AS is_reset,
			       multiIf(is_first, toUInt64(0), is_reset, hcount, toUInt64(hcount - lagInFrame(hcount) OVER w)) AS dcount,
			       multiIf(is_first, 0., is_reset, hsum, hsum - lagInFrame(hsum) OVER w) AS dsum,
			       multiIf(is_first, arrayMap(c -> toUInt64(0), counts),
			               is_reset, counts,
			               arrayMap((c, p) -> toUInt64(if(c > p, c - p, 0)), counts, lagInFrame(counts) OVER w)) AS dcounts
			FROM per_series
			WINDOW w AS (
				PARTITION BY fingerprint
				ORDER BY sample_at
				ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
			)
		)
		SELECT ` + resultCols + `,
		       sum(dsum)   AS hist_sum,
		       sum(dcount) AS hist_count,
		       quantilesPrometheusHistogramArray(0.5, 0.95, 0.99)(
		           if(length(dcounts) = length(bounds) + 1, arrayPushBack(bounds, inf), emptyArrayFloat64()),
		           if(length(dcounts) = length(bounds) + 1, arrayCumSum(dcounts), emptyArrayUInt64())
		       ) AS quantiles
		FROM increases
		GROUP BY ` + resultCols + `
		HAVING bucket_at >= @displayStart
		ORDER BY bucket_at ASC
		SETTINGS max_execution_time = 30`
}

func metricArgs(f filter.Filters) []any {
	return append(chargs.RangeArgs(f.TenantID, f.StartMs, f.EndMs), clickhouse.Named("metricName", f.MetricName))
}

package explorer

import (
	"context"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/metrics/filter"
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
		       argMax(metric_type, (timestamp, fingerprint)) AS metric_type,
		       argMax(unit, (timestamp, fingerprint))        AS unit,
		       argMax(description, (timestamp, fingerprint)) AS description
		FROM optikk.metrics_series
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start - INTERVAL 1 HOUR AND timestamp < @end
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
	col := seriesResourceColumn(canonical)
	if col == "" {
		return nil, nil
	}
	query := `
		SELECT ` + col + ` AS tag_value,
		       uniqExact(fingerprint) AS count
		FROM optikk.metrics_series
		PREWHERE tenant_id     = @tenantID
		     AND timestamp >= @start - INTERVAL 1 HOUR AND timestamp < @end
		     AND metric_name = @metricName
		WHERE ` + col + ` != ''
		GROUP BY tag_value
		ORDER BY count DESC, tag_value ASC
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
		       uniqExact(fingerprint) AS count
		FROM optikk.metrics_series
		PREWHERE tenant_id     = @tenantID
		     AND timestamp >= @start - INTERVAL 1 HOUR AND timestamp < @end
		     AND metric_name = @metricName
		WHERE ` + col + ` != ''
		GROUP BY tag_value
		ORDER BY count DESC, tag_value ASC
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
	query := `
		SELECT kv.1    AS tag_key,
		       kv.2    AS tag_value,
		       uniqExact(fingerprint) AS count
		FROM optikk.metrics_series
		ARRAY JOIN arrayConcat(
		    CAST(attributes, 'Array(Tuple(String, String))'),
		    [('service', toString(service)), ('host', toString(host)),
		     ('pod', toString(pod)), ('container', toString(container)),
		     ('environment', toString(environment)), ('k8s_namespace', toString(k8s_namespace)),
		     ('k8s.node.name', resource_attributes['k8s.node.name']),
		     ('cloud.provider', resource_attributes['cloud.provider']),
		     ('cloud.account.id', resource_attributes['cloud.account.id']),
		     ('cloud.region', resource_attributes['cloud.region']),
		     ('cloud.platform', resource_attributes['cloud.platform'])]
		) AS kv
		PREWHERE tenant_id     = @tenantID
		     AND timestamp >= @start - INTERVAL 1 HOUR AND timestamp < @end
		     AND metric_name = @metricName
		WHERE kv.2 != ''
		GROUP BY tag_key, tag_value
		ORDER BY tag_key, count DESC, tag_value ASC
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
		       argMax(ms.temporality, (timestamp, fingerprint))  AS temporality,
		       argMax(ms.is_monotonic, (timestamp, fingerprint)) AS is_monotonic,
		       argMax(ms.metric_type, (timestamp, fingerprint))  AS metric_type,
		       uniqExact((ms.temporality, ms.is_monotonic, ms.metric_type)) AS variants
		FROM optikk.metrics_series AS ms
		PREWHERE tenant_id     = @tenantID
		     AND metric_name IN @metricNames
		     AND timestamp >= @start - INTERVAL 1 HOUR AND timestamp < @end
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
	displayStart := f.StartMs
	if f.Cumulative {
		f.StartMs = displayStart - int64(time.Hour/time.Millisecond)
	}
	fromTable, where, selectCols, groupByCols, filterArgs := filter.BuildSelection(f)

	var query string
	switch {
	case f.Histogram && strings.HasPrefix(f.Aggregation, "p"):
		query = histogramQuantileQuery(fromTable, where, selectCols, groupByCols)
	case f.Cumulative:
		fromTable = "optikk.metrics"
		query = cumulativeRollupQuery(fromTable, where, selectCols, groupByCols, len(f.GroupBy) > 0)
	default:
		query = deltaRollupQuery(fromTable, where, selectCols, groupByCols)
	}

	args := append(metricArgs(f), filterArgs...)
	if f.Cumulative {
		args = append(args, clickhouse.Named("displayStart", time.UnixMilli(displayStart)))
	}
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
		     AND timestamp >= @start AND timestamp < @end` + where + `
		GROUP BY ` + groupByCols + `
		ORDER BY bucket_at ASC
		SETTINGS max_execution_time = 30`
}

func histogramQuantileQuery(fromTable, where, selectCols, groupByCols string) string {
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

func cumulativeRollupQuery(fromTable, where, selectCols, groupByCols string, grouped bool) string {
	resultCols := "bucket_at"
	if grouped {
		resultCols += ", group_values"
	}
	perSeries := `
		SELECT fingerprint, timestamp AS sample_at, ` + selectCols + `,
		       max(value) AS cval
		FROM ` + fromTable + `
		PREWHERE tenant_id     = @tenantID
		     AND metric_name = @metricName
		     AND timestamp >= @start AND timestamp < @end` + where + `
		GROUP BY fingerprint, sample_at, ` + groupByCols + ``

	increase := `
		SELECT ` + resultCols + `,
		       if(row_number() OVER w = 1, 0,
		          if(cval < lagInFrame(cval) OVER w, cval,
		             cval - lagInFrame(cval) OVER w)) AS increase
		FROM (` + perSeries + `)
		WINDOW w AS (PARTITION BY fingerprint ORDER BY sample_at)`

	return `
		SELECT ` + resultCols + `,
		       sum(increase) AS val_sum,
		       toUInt64(0)   AS val_count,
		       toFloat64(0)  AS val_min,
		       toFloat64(0)  AS val_max
		FROM (` + increase + `)
		GROUP BY ` + resultCols + `
		HAVING bucket_at >= @displayStart
		ORDER BY bucket_at ASC
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

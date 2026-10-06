package query

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/shared/chargs"
)

type MetricBackend struct {
	db clickhouse.Conn
}

func NewMetricBackend(db clickhouse.Conn) *MetricBackend { return &MetricBackend{db: db} }

func (b *MetricBackend) Scalar(ctx context.Context, m models.MonitorRow, now time.Time) (ScalarResult, error) {
	q := m.Query.Metric
	if q == nil {
		return ScalarResult{}, errMissingQuery(m)
	}
	windowSec := int64(q.WindowSec)
	startMs, endMs := completeWindow(now, windowSec, timebucket.RollupGrainSeconds(windowSec*1000))

	scopeSQL, args, err := CompileScope("metric", m.Scope, metricArgs(m.TenantID, q, startMs, endMs))
	if err != nil {
		return ScalarResult{}, err
	}
	var query string
	if q.Aggregation == "sum" {
		query = metricSumQuery("", scopeSQL)
	} else {
		expr := metricSources[q.Aggregation]
		samples, _ := metricGuards(q.Aggregation, expr)
		query = `
		SELECT ` + samples + ` AS samples, ` + expr + ` AS value
		FROM ` + timebucket.MetricsRollup(startMs, endMs) + `
		PREWHERE tenant_id     = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND metric_name = @metricName` + scopeSQL
	}
	var row scalarRow
	if err := dbutil.QueryRowCH(dbutil.DashboardCtx(ctx), b.db, "alerting.metric.Scalar", &row, query, args...); err != nil {
		return ScalarResult{}, err
	}
	return ScalarResult{Value: row.Value, HasData: row.Samples > 0}, nil
}

func (b *MetricBackend) Series(ctx context.Context, m models.MonitorRow, windowMs int64, now time.Time) ([]Point, error) {
	q := m.Query.Metric
	if q == nil {
		return nil, errMissingQuery(m)
	}
	endMs := now.UnixMilli()
	startMs := endMs - windowMs

	scopeSQL, args, err := CompileScope("metric", m.Scope, metricArgs(m.TenantID, q, startMs, endMs))
	if err != nil {
		return nil, err
	}
	bucketSQL := timebucket.DisplayGrainSQL(windowMs)
	var query string
	if q.Aggregation == "sum" {
		query = metricSumQuery(bucketSQL, scopeSQL)
	} else {
		expr := metricSources[q.Aggregation]
		_, having := metricGuards(q.Aggregation, expr)
		query = `
		SELECT ` + bucketSQL + ` AS bucket, ` + expr + ` AS value
		FROM ` + timebucket.MetricsRollup(startMs, endMs) + `
		PREWHERE tenant_id     = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND metric_name = @metricName` + scopeSQL + `
		GROUP BY bucket` + having + `
		ORDER BY bucket`
	}

	var rows []bucketRow
	if err := dbutil.SelectCH(dbutil.DashboardCtx(ctx), b.db, "alerting.metric.Series", &rows, query, args...); err != nil {
		return nil, err
	}
	out := make([]Point, 0, len(rows))
	for _, r := range rows {
		out = append(out, Point{BucketMs: r.Bucket.UnixMilli(), Value: r.Value})
	}
	return out, nil
}

// metricSources is the value expression per aggregation, except sum, which
// metricSumQuery derives from counter increases.
var metricSources = map[string]string{
	"avg": "if(sum(hist_count) > 0, sum(hist_sum) / nullIf(sum(hist_count), 0), sum(val_sum) / nullIf(sum(val_count), 0))",
	"min": "min(val_min)", "max": "max(val_max)",
	"p50": "(quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(latency_state))[1]",
	"p95": "(quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(latency_state))[2]",
	"p99": "(quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(latency_state))[3]",
}

const metricHistogramProbe = "isFinite((quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(latency_state))[1])"

func metricSumRows(scopeSQL string) string {
	current := "if(hcount > 0 OR hsum != 0, hsum, cval)"
	return `
		SELECT *, if(temporality = 'Cumulative',
		       if(rn = 1, 0, if(` + current + ` < previous, ` + current + `, ` + current + ` - previous)), ` + current + `) AS increase
		FROM (
		    SELECT *, row_number() OVER w AS rn, lagInFrame(` + current + `) OVER w AS previous
		    FROM (
		        SELECT fingerprint, timestamp, any(temporality) AS temporality, max(value) AS cval,
		               max(hist_sum) AS hsum, max(hist_count) AS hcount
		        FROM optikk.metrics
		        PREWHERE tenant_id = @tenantID AND metric_name = @metricName
		             AND timestamp >= @start AND timestamp < @end` + scopeSQL + `
		        GROUP BY fingerprint, timestamp
		    )
		    WINDOW w AS (PARTITION BY fingerprint ORDER BY timestamp)
		)`
}

func metricSumQuery(bucketSQL, scopeSQL string) string {
	if bucketSQL == "" {
		return `SELECT countIf(timestamp >= @displayStart) AS samples,
		               sumIf(increase, timestamp >= @displayStart) AS value
		        FROM (` + metricSumRows(scopeSQL) + `)`
	}
	return `SELECT ` + bucketSQL + ` AS bucket, sum(increase) AS value FROM (` +
		metricSumRows(scopeSQL) + `) WHERE timestamp >= @displayStart GROUP BY bucket ORDER BY bucket`
}

func metricGuards(agg, expr string) (samples, having string) {
	samples = "greatest(sum(val_count), sum(hist_count))"
	switch agg {
	case "p50", "p95", "p99":
		return "if(countIf(temporality = 'Cumulative') = 0 AND isFinite(" + expr + "), sum(hist_count), 0)",
			" HAVING sum(hist_count) > 0 AND countIf(temporality = 'Cumulative') = 0 AND isFinite(" + expr + ")"
	case "min", "max":
		return "if(sum(hist_count) > 0, 0, sum(val_count))", " HAVING sum(hist_count) = 0"
	case "avg":
		invalid := "countIf(temporality = 'Cumulative') > 0 OR NOT " + metricHistogramProbe
		return "if(sum(hist_count) > 0 AND (" + invalid + "), 0, " + samples + ")",
			" HAVING sum(hist_count) = 0 OR NOT (" + invalid + ")"
	default:
		return samples, ""
	}
}

// metricArgs binds the display window. Sums read an extra hour of raw
// samples before it (@start) so the first displayed counter increase has a
// previous sample to diff against (@displayStart).
func metricArgs(tenantID int64, q *models.MetricQuery, startMs, endMs int64) []any {
	sourceStart := startMs
	if q.Aggregation == "sum" {
		sourceStart -= time.Hour.Milliseconds()
	}
	return []any{
		tenantIDArg(tenantID),
		clickhouse.Named("metricName", q.Metric),
		chargs.Millis("start", sourceStart),
		chargs.Millis("end", endMs),
		chargs.Millis("displayStart", startMs),
	}
}

type scalarRow struct {
	Samples uint64  `ch:"samples"`
	Value   float64 `ch:"value"`
}

type bucketRow struct {
	Bucket time.Time `ch:"bucket"`
	Value  float64   `ch:"value"`
}

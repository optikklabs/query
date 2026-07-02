package query

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

// MetricBackend evaluates metric monitors against ClickHouse raw metrics.
type MetricBackend struct {
	db clickhouse.Conn
}

func NewMetricBackend(db clickhouse.Conn) *MetricBackend { return &MetricBackend{db: db} }

func (b *MetricBackend) Scalar(ctx context.Context, m models.MonitorRow, q models.MonitorQuery, scope models.Scope, _ models.Conditions, now time.Time) (ScalarResult, error) {
	if q.Metric == nil {
		return ScalarResult{}, nil
	}
	windowSec := int64(q.Metric.WindowSec)
	if windowSec <= 0 {
		windowSec = 300
	}
	endMs := now.UnixMilli()
	startMs := endMs - windowSec*1000

	table, expr := metricSource(q.Metric.Aggregation)
	query := `
		SELECT ` + expr + ` AS value
		FROM optikk.` + table + `
		PREWHERE team_id     = @teamID
		     AND metric_name = @metricName
		WHERE timestamp BETWEEN @start AND @end`

	args := metricArgs(m.TeamID, q.Metric.Metric, startMs, endMs)
	var rows []scalarRow
	if err := dbutil.SelectCH(dbutil.DashboardCtx(ctx), b.db, "alerting.metric.Scalar", &rows, query, args...); err != nil {
		return ScalarResult{}, err
	}
	if len(rows) == 0 {
		return ScalarResult{}, nil
	}
	r := rows[0]
	return ScalarResult{Value: r.Value, HasData: !r.IsZeroNoData()}, nil
}

func (b *MetricBackend) Series(ctx context.Context, m models.MonitorRow, q models.MonitorQuery, scope models.Scope, _ models.Conditions, windowMs int64, now time.Time) ([]Point, error) {
	if q.Metric == nil {
		return nil, nil
	}
	endMs := now.UnixMilli()
	startMs := endMs - windowMs

	table, expr := metricSource(q.Metric.Aggregation)
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(windowMs) + ` AS bucket, ` +
		expr + ` AS value
		FROM optikk.` + table + `
		PREWHERE team_id     = @teamID
		     AND metric_name = @metricName
		WHERE timestamp BETWEEN @start AND @end
		GROUP BY bucket
		ORDER BY bucket`

	args := metricArgs(m.TeamID, q.Metric.Metric, startMs, endMs)
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

func metricSource(agg string) (table, expr string) {
	switch agg {
	case "sum":
		return "metrics", "sum(value)"
	case "min":
		return "metrics", "min(value)"
	case "max":
		return "metrics", "max(value)"
	case "p50":
		return "metrics", "(quantilesPrometheusHistogramArray(0.50, 0.95, 0.99)(hist_buckets, arrayCumSum(hist_counts)))[1]"
	case "p95":
		return "metrics", "(quantilesPrometheusHistogramArray(0.50, 0.95, 0.99)(hist_buckets, arrayCumSum(hist_counts)))[2]"
	case "p99":
		return "metrics", "(quantilesPrometheusHistogramArray(0.50, 0.95, 0.99)(hist_buckets, arrayCumSum(hist_counts)))[3]"
	default:
		return "metrics", "avg(value)"
	}
}

func metricArgs(teamID int64, metricName string, startMs, endMs int64) []any {
	return []any{
		teamIDArg(teamID),
		clickhouse.Named("metricName", metricName),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
	}
}

type scalarRow struct {
	Value float64 `ch:"value"`
}

func (s scalarRow) IsZeroNoData() bool { return false }

type bucketRow struct {
	Bucket time.Time `ch:"bucket"`
	Value  float64   `ch:"value"`
}

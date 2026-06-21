package query

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/seriesattr"
)

// durationStatusCTE resolves the spanmetrics status_code per fingerprint for the
// 'duration' histogram, joined to the rollup on fingerprint.
const durationStatusCTE = `
		WITH series AS (
		    SELECT fingerprint,
		           any(service)                       AS service,
		           any(` + seriesattr.StatusCode + `) AS status_code
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE service = @service
		    GROUP BY fingerprint
		)`

// APMBackend evaluates APM monitors against spanmetrics.
type APMBackend struct {
	db clickhouse.Conn
}

func NewAPMBackend(db clickhouse.Conn) *APMBackend { return &APMBackend{db: db} }

func (b *APMBackend) Scalar(ctx context.Context, m models.MonitorRow, q models.MonitorQuery, _ models.Scope, cond models.Conditions, now time.Time) (ScalarResult, error) {
	if q.APM == nil {
		return ScalarResult{}, nil
	}
	windowSec := int64(q.APM.WindowSec)
	if windowSec <= 0 {
		windowSec = 300
	}
	endMs := now.UnixMilli()
	startMs := endMs - windowSec*1000

	const query = durationStatusCTE + `
		SELECT sum(m.hist_count)                                          AS request_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)    AS error_count,
		       quantilesPrometheusHistogramMerge(0.99)(quantilesPrometheusHistogramArrayState(0.99)(m.hist_buckets, arrayCumSum(m.hist_counts))) AS qs
		FROM optikk.metrics AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.metric_name = 'traces.span.metrics.duration'
		WHERE m.timestamp BETWEEN @start AND @end`

	args := apmArgs(m.TeamID, q.APM.Service, startMs, endMs)
	var rows []apmAggRow
	if err := dbutil.SelectCH(dbutil.DashboardCtx(ctx), b.db, "alerting.apm.Scalar", &rows, query, args...); err != nil {
		return ScalarResult{}, err
	}
	if len(rows) == 0 || rows[0].RequestCount == 0 {
		return ScalarResult{HasData: false}, nil
	}
	row := rows[0]
	if len(row.QS) > 0 {
		row.P99 = row.QS[0]
	}

	if cond.MinSample != nil && row.RequestCount < uint64(*cond.MinSample) {
		return ScalarResult{HasData: false}, nil
	}

	value := apmTrackValue(q.APM.Track, row, windowSec)
	return ScalarResult{Value: value, HasData: true}, nil
}

func (b *APMBackend) Series(ctx context.Context, m models.MonitorRow, q models.MonitorQuery, _ models.Scope, _ models.Conditions, windowMs int64, now time.Time) ([]Point, error) {
	if q.APM == nil {
		return nil, nil
	}
	endMs := now.UnixMilli()
	startMs := endMs - windowMs
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)

	query := durationStatusCTE + `
		SELECT ` + timebucket.DisplayGrainSQL(windowMs) + ` AS bucket,
		       sum(m.hist_count)                                          AS request_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)    AS error_count,
		       quantilesPrometheusHistogramMerge(0.99)(quantilesPrometheusHistogramArrayState(0.99)(m.hist_buckets, arrayCumSum(m.hist_counts))) AS qs
		FROM optikk.metrics AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.metric_name = 'traces.span.metrics.duration'
		WHERE m.timestamp BETWEEN @start AND @end
		GROUP BY bucket
		ORDER BY bucket`

	args := apmArgs(m.TeamID, q.APM.Service, startMs, endMs)
	var rows []apmSeriesRow
	if err := dbutil.SelectCH(dbutil.DashboardCtx(ctx), b.db, "alerting.apm.Series", &rows, query, args...); err != nil {
		return nil, err
	}
	out := make([]Point, 0, len(rows))
	windowSec := int64(q.APM.WindowSec)
	if windowSec <= 0 {
		windowSec = 300
	}
	for _, r := range rows {
		var p99 float64
		if len(r.QS) > 0 {
			p99 = r.QS[0]
		}
		row := apmAggRow{RequestCount: r.RequestCount, ErrorCount: r.ErrorCount, P99: p99}
		out = append(out, Point{BucketMs: r.Bucket.UnixMilli(), Value: apmTrackValue(q.APM.Track, row, windowSec)})
	}
	return out, nil
}

// apmTrackValue projects the requested track from the aggregate row.
func apmTrackValue(track string, row apmAggRow, windowSec int64) float64 {
	switch track {
	case "errors":
		if row.RequestCount == 0 {
			return 0
		}
		return float64(row.ErrorCount) / float64(row.RequestCount) * 100
	case "hits":
		if windowSec == 0 {
			return float64(row.RequestCount)
		}
		return float64(row.RequestCount) / float64(windowSec)
	case "latency":
		return row.P99
	case "apdex":
		// Apdex is not yet supported and returns 0.
		return 0
	}
	return 0
}

func apmArgs(teamID int64, service string, startMs, endMs int64) []any {
	bs, be := chargs.BucketBounds(startMs, endMs)
	return []any{
		teamIDArg(teamID),
		clickhouse.Named("bucketStart", bs),
		clickhouse.Named("bucketEnd", be),
		clickhouse.Named("service", service),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
	}
}

type apmAggRow struct {
	RequestCount uint64    `ch:"request_count"`
	ErrorCount   uint64    `ch:"error_count"`
	QS           []float64 `ch:"qs"`
	P99          float64   `ch:"p99"`
}

type apmSeriesRow struct {
	Bucket       time.Time `ch:"bucket"`
	RequestCount uint64    `ch:"request_count"`
	ErrorCount   uint64    `ch:"error_count"`
	QS           []float64 `ch:"qs"`
	P99          float64   `ch:"p99"`
}

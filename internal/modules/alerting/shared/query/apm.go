package query

import (
	"context"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/metrics"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type APMBackend struct {
	db clickhouse.Conn
}

func NewAPMBackend(db clickhouse.Conn) *APMBackend { return &APMBackend{db: db} }

func (b *APMBackend) Scalar(ctx context.Context, m models.MonitorRow, now time.Time) (ScalarResult, error) {
	q := m.Query.APM
	if q == nil {
		return ScalarResult{}, errMissingQuery(m)
	}
	windowSec := int64(q.WindowSec)
	startMs, endMs := completeWindow(now, windowSec, 60)

	query := `
		SELECT ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.LatencyP99.SQL() + `
		FROM optikk.span_stats_1m
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND service = @service
		     AND (@resource = '' OR span_name = @resource)
		     AND ` + spanstats.InboundPred

	scopeSQL, args, err := CompileScope("apm", m.Scope, apmArgs(m.TenantID, *q, startMs, endMs))
	if err != nil {
		return ScalarResult{}, err
	}
	query += scopeSQL
	var row apmAggRow
	if err := dbutil.QueryRowCH(dbutil.DashboardCtx(ctx), b.db, "alerting.apm.Scalar", &row, query, args...); err != nil {
		return ScalarResult{}, err
	}
	if row.RequestCount == 0 {
		return ScalarResult{HasData: false}, nil
	}
	row.P99 = spanstats.LatencyP99.At(row.QS, spanstats.P99)

	if minSample := m.Conditions.MinSample; minSample != nil && row.RequestCount < uint64(*minSample) {
		return ScalarResult{HasData: false}, nil
	}

	value := apmTrackValue(q.Track, row, windowSec)
	return ScalarResult{Value: value, HasData: true}, nil
}

func (b *APMBackend) Series(ctx context.Context, m models.MonitorRow, windowMs int64, now time.Time) ([]Point, error) {
	q := m.Query.APM
	if q == nil {
		return nil, errMissingQuery(m)
	}
	endMs := now.UnixMilli()
	startMs := endMs - windowMs

	scopeSQL, args, err := CompileScope("apm", m.Scope, apmArgs(m.TenantID, *q, startMs, endMs))
	if err != nil {
		return nil, err
	}
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(windowMs) + ` AS bucket,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.LatencyP99.SQL() + `
		FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND service = @service
		     AND (@resource = '' OR span_name = @resource)
		     AND ` + spanstats.InboundPred + scopeSQL + `
		GROUP BY bucket
		ORDER BY bucket`

	var rows []apmSeriesRow
	if err := dbutil.SelectCH(dbutil.DashboardCtx(ctx), b.db, "alerting.apm.Series", &rows, query, args...); err != nil {
		return nil, err
	}
	out := make([]Point, 0, len(rows))
	bucketSec := int64(timebucket.DisplayGrain(windowMs).Seconds())
	for _, r := range rows {
		p99 := spanstats.LatencyP99.At(r.QS, spanstats.P99)
		row := apmAggRow{RequestCount: r.RequestCount, ErrorCount: r.ErrorCount, P99: p99}
		out = append(out, Point{BucketMs: r.Bucket.UnixMilli(), Value: apmTrackValue(q.Track, row, bucketSec)})
	}
	return out, nil
}

// apmTrackValue is the tracked signal: error percentage, requests per
// second, or p99 latency (ms) for the "latency" track.
func apmTrackValue(track string, row apmAggRow, windowSec int64) float64 {
	switch track {
	case "errors":
		return metrics.Percentage(row.ErrorCount, row.RequestCount)
	case "hits":
		return float64(row.RequestCount) / float64(windowSec)
	default:
		return row.P99
	}
}

func apmArgs(tenantID int64, q models.APMQuery, startMs, endMs int64) []any {
	return []any{
		tenantIDArg(tenantID),
		clickhouse.Named("service", q.Service),
		clickhouse.Named("resource", strings.TrimSpace(q.Resource)),
		chargs.Millis("start", startMs),
		chargs.Millis("end", endMs),
	}
}

type apmAggRow struct {
	RequestCount uint64    `ch:"request_total"`
	ErrorCount   uint64    `ch:"error_total"`
	QS           []float64 `ch:"qs"`
	P99          float64   `ch:"p99"`
}

type apmSeriesRow struct {
	Bucket       time.Time `ch:"bucket"`
	RequestCount uint64    `ch:"request_total"`
	ErrorCount   uint64    `ch:"error_total"`
	QS           []float64 `ch:"qs"`
}

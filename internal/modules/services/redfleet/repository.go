package redfleet

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/seriesattr"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetFleetREDMetrics(ctx context.Context, teamID int64, startMs, endMs int64) ([]redMetricsRow, error) {
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           any(service)                       AS service,
		           any(` + seriesattr.StatusCode + `) AS status_code
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE ` + seriesattr.ServerKindPred + `
		    GROUP BY fingerprint
		)
		SELECT series.service                                              AS service,
		       sum(m.hist_count)                                           AS total_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)     AS error_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state) AS qs
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY service`
	var rows []redMetricsRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetFleetREDMetrics",
		&rows, query, chargs.RollupRangeArgs(teamID, startMs, endMs)...); err != nil {
		return nil, err
	}
	for i := range rows {
		if len(rows[i].QS) >= 3 {
			rows[i].P50Ms = float32(rows[i].QS[0])
			rows[i].P95Ms = float32(rows[i].QS[1])
			rows[i].P99Ms = float32(rows[i].QS[2])
		}
	}
	return rows, nil
}

func (r *Repository) GetApdex(ctx context.Context, teamID int64, startMs, endMs int64, satisfiedMs, toleratingMs float64) ([]apdexRow, error) {
	const query = `
		WITH active_fps AS (
		    SELECT DISTINCT fingerprint
		    FROM optikk.spans_resource
		    PREWHERE team_id   = @teamID
		         AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		)
		SELECT service                                                              AS service,
		       count()                                                              AS total_count,
		       countIf(duration_nano <= @satisfiedNs)                               AS satisfied,
		       countIf(duration_nano > @satisfiedNs AND duration_nano <= @toleratingNs) AS tolerating
		FROM optikk.spans
		PREWHERE team_id     = @teamID
		     AND ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		     AND fingerprint IN active_fps
		     AND timestamp   BETWEEN @start AND @end
		GROUP BY service
		ORDER BY total_count DESC`
	args := append(chargs.RangeArgs(teamID, startMs, endMs),
		clickhouse.Named("satisfiedNs", uint64(satisfiedMs*1_000_000)),
		clickhouse.Named("toleratingNs", uint64(toleratingMs*1_000_000)),
	)
	var rows []apdexRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetApdex",
		&rows, query, args...)
}

func (r *Repository) GetApdexByService(ctx context.Context, teamID int64, startMs, endMs int64, satisfiedMs, toleratingMs float64, serviceName string) ([]apdexRow, error) {
	const query = `
		WITH active_fps AS (
		    SELECT DISTINCT fingerprint
		    FROM optikk.spans_resource
		    PREWHERE team_id   = @teamID
		         AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		         AND service   = @serviceName
		)
		SELECT service                                                              AS service,
		       count()                                                              AS total_count,
		       countIf(duration_nano <= @satisfiedNs)                               AS satisfied,
		       countIf(duration_nano > @satisfiedNs AND duration_nano <= @toleratingNs) AS tolerating
		FROM optikk.spans
		PREWHERE team_id     = @teamID
		     AND ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		     AND fingerprint IN active_fps
		     AND timestamp   BETWEEN @start AND @end
		WHERE service   = @serviceName
		GROUP BY service
		ORDER BY total_count DESC`
	args := append(chargs.RangeArgs(teamID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("satisfiedNs", uint64(satisfiedMs*1_000_000)),
		clickhouse.Named("toleratingNs", uint64(toleratingMs*1_000_000)),
	)
	var rows []apdexRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetApdexByService",
		&rows, query, args...)
}

type requestRateRawRow struct {
	BucketAt     time.Time `ch:"bucket_at"`
	RequestCount uint64    `ch:"request_count"`
	ErrorCount   uint64    `ch:"error_count"`
}

func (r *Repository) GetRequestAndErrorRateTimeSeries(ctx context.Context, teamID int64, startMs, endMs int64) ([]requestRateRawRow, error) {
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           any(` + seriesattr.StatusCode + `) AS status_code
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE ` + seriesattr.ServerKindPred + `
		    GROUP BY fingerprint
		)
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       sum(m.hist_count)                                       AS request_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `) AS error_count
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	var rows []requestRateRawRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetRequestAndErrorRateTimeSeries",
		&rows, query, chargs.RollupRangeArgs(teamID, startMs, endMs)...)
}

// statusBucketTimeseriesRow is one (bucket, status-class) row from spanmetrics.
type statusBucketTimeseriesRow struct {
	BucketAt     time.Time `ch:"bucket_at"`
	StatusBucket string    `ch:"http_status_bucket"`
	RequestCount uint64    `ch:"request_count"`
}

// latencyPercentilesTimeseriesRow holds p50/p95/p99 for one display bucket.
type latencyPercentilesTimeseriesRow struct {
	BucketAt time.Time `ch:"bucket_at"`
	QS       []float64 `ch:"qs"`
	P50Ms    float32   `ch:"p50_ms"`
	P95Ms    float32   `ch:"p95_ms"`
	P99Ms    float32   `ch:"p99_ms"`
}

// endpointRateRow is one (bucket, route) RED sample from spanmetrics, feeding
// the per-endpoint golden-signal lines.
type endpointRateRow struct {
	BucketAt     time.Time `ch:"bucket_at"`
	HTTPRoute    string    `ch:"http_route"`
	RequestCount uint64    `ch:"request_count"`
	ErrorCount   uint64    `ch:"error_count"`
	QS           []float64 `ch:"qs"`
}

// topDBQueryRow combines rate/error/percentile shape for one DB query operation.
type topDBQueryRow struct {
	ServiceName   string    `ch:"service"`
	OperationName string    `ch:"operation_name"`
	DBSystem      string    `ch:"db_system"`
	TotalCount    uint64    `ch:"total_count"`
	ErrorCount    uint64    `ch:"error_count"`
	QS            []float64 `ch:"qs"`
	P50Ms         float32   `ch:"p50_ms"`
	P95Ms         float32   `ch:"p95_ms"`
	P99Ms         float32   `ch:"p99_ms"`
}

// topEndpointRow combines rate/error/percentile shape for one operation.
type topEndpointRow struct {
	ServiceName   string    `ch:"service"`
	OperationName string    `ch:"operation_name"`
	SpanKind      string    `ch:"kind_string"`
	HTTPRoute     string    `ch:"http_route"`
	TotalCount    uint64    `ch:"total_count"`
	ErrorCount    uint64    `ch:"error_count"`
	QS            []float64 `ch:"qs"`
	P50Ms         float32   `ch:"p50_ms"`
	P95Ms         float32   `ch:"p95_ms"`
	P99Ms         float32   `ch:"p99_ms"`
}

func (r *Repository) GetStatusTimeSeries(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string,
) ([]statusBucketTimeseriesRow, error) {
	grainSQL := timebucket.DisplayGrainSQL(endMs - startMs)
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           any(` + seriesattr.HTTPStatusCode + `) AS http_status_code
		    FROM optikk.metrics_series AS s
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE 1=1
		          ` + serviceWherePred(serviceName) + `
		          AND ` + seriesattr.ServerKindPred + `
		    GROUP BY fingerprint
		)
		SELECT ` + grainSQL + ` AS bucket_at,
		       series.http_status_code AS http_status_bucket,
		       sum(m.hist_count)       AS request_count
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY bucket_at, http_status_bucket
		ORDER BY bucket_at ASC`
	var rows []statusBucketTimeseriesRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetStatusTimeSeries",
		&rows, query, detailArgs(teamID, startMs, endMs, serviceName)...)
}

func (r *Repository) GetLatencyPercentilesTimeSeries(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string,
) ([]latencyPercentilesTimeseriesRow, error) {
	grainSQL := timebucket.DisplayGrainSQL(endMs - startMs)
	query := `
		WITH series AS (
		    SELECT fingerprint
		    FROM optikk.metrics_series AS s
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE 1=1
		          ` + serviceWherePred(serviceName) + `
		          AND ` + seriesattr.ServerKindPred + `
		    GROUP BY fingerprint
		)
		SELECT ` + grainSQL + ` AS bucket_at,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state) AS qs
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	var rows []latencyPercentilesTimeseriesRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetLatencyPercentilesTimeSeries",
		&rows, query, detailArgs(teamID, startMs, endMs, serviceName)...); err != nil {
		return nil, err
	}
	for i := range rows {
		if len(rows[i].QS) >= 3 {
			rows[i].P50Ms = float32(rows[i].QS[0])
			rows[i].P95Ms = float32(rows[i].QS[1])
			rows[i].P99Ms = float32(rows[i].QS[2])
		}
	}
	return rows, nil
}

// GetREDByEndpointTimeSeries returns request/error counts and p50/p95/p99 per
// (display-bucket, http.route) for SERVER spans of one service.
func (r *Repository) GetREDByEndpointTimeSeries(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string,
) ([]endpointRateRow, error) {
	grainSQL := timebucket.DisplayGrainSQL(endMs - startMs)
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           any(` + seriesattr.HTTPRoute + `)  AS http_route,
		           any(` + seriesattr.StatusCode + `) AS status_code
		    FROM optikk.metrics_series AS s
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE 1=1
		          ` + serviceWherePred(serviceName) + `
		          AND ` + seriesattr.ServerKindPred + `
		    GROUP BY fingerprint
		)
		SELECT ` + grainSQL + ` AS bucket_at,
		       series.http_route                                                  AS http_route,
		       sum(m.hist_count)                                                   AS request_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)             AS error_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state) AS qs
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY bucket_at, http_route
		ORDER BY bucket_at ASC`
	var rows []endpointRateRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetREDByEndpointTimeSeries",
		&rows, query, detailArgs(teamID, startMs, endMs, serviceName)...)
}

func (r *Repository) GetTopEndpointsCombined(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string, limit int, cursor TopEndpointsCursor,
) ([]topEndpointRow, error) {
	var paginationFilter string
	if !cursor.IsZero() {
		paginationFilter = "AND (total_count < @cursorCount OR (total_count = @cursorCount AND operation_name > @cursorOp))"
	}

	query := `
		WITH series AS (
		    SELECT fingerprint,
		           any(service)                       AS service,
		           any(` + seriesattr.SpanName + `)   AS span_name,
		           any(` + seriesattr.SpanKind + `)   AS span_kind,
		           any(` + seriesattr.HTTPRoute + `)  AS http_route,
		           any(` + seriesattr.StatusCode + `) AS status_code
		    FROM optikk.metrics_series AS s
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE 1=1
		          ` + serviceWherePred(serviceName) + `
		          AND ` + seriesattr.ServerKindPred + `
		    GROUP BY fingerprint
		)
		SELECT any(series.service)                                                  AS service,
		       series.span_name                                                     AS operation_name,
		       any(series.span_kind)                                                AS kind_string,
		       any(series.http_route)                                               AS http_route,
		       sum(m.hist_count)                                                    AS total_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)              AS error_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state)  AS qs
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY operation_name
		HAVING operation_name != '' ` + paginationFilter + `
		ORDER BY total_count DESC, operation_name ASC
		LIMIT @limit`
	args := append(detailArgs(teamID, startMs, endMs, serviceName),
		clickhouse.Named("limit", limit),
		clickhouse.Named("cursorCount", cursor.TotalCount),
		clickhouse.Named("cursorOp", cursor.OperationName),
	)
	var rows []topEndpointRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetTopEndpointsCombined",
		&rows, query, args...); err != nil {
		return nil, err
	}
	for i := range rows {
		if len(rows[i].QS) >= 3 {
			rows[i].P50Ms = float32(rows[i].QS[0])
			rows[i].P95Ms = float32(rows[i].QS[1])
			rows[i].P99Ms = float32(rows[i].QS[2])
		}
	}
	return rows, nil
}

func (r *Repository) GetTopDBQueriesCombined(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string, limit int, cursor TopEndpointsCursor,
) ([]topDBQueryRow, error) {
	var paginationFilter string
	if !cursor.IsZero() {
		paginationFilter = "AND (total_count < @cursorCount OR (total_count = @cursorCount AND operation_name > @cursorOp))"
	}

	query := `
		WITH series AS (
		    SELECT fingerprint,
		           any(service)                       AS service,
		           any(` + seriesattr.SpanName + `)   AS span_name,
		           any(` + seriesattr.DBSystem + `)   AS db_system,
		           any(` + seriesattr.StatusCode + `) AS status_code
		    FROM optikk.metrics_series AS s
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE 1=1
		          ` + serviceWherePred(serviceName) + `
		          AND ` + seriesattr.DBSpanPred + `
		    GROUP BY fingerprint
		)
		SELECT any(series.service)                                                  AS service,
		       series.span_name                                                     AS operation_name,
		       any(series.db_system)                                                AS db_system,
		       sum(m.hist_count)                                                    AS total_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)              AS error_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state)  AS qs
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY operation_name
		HAVING operation_name != '' ` + paginationFilter + `
		ORDER BY total_count DESC, operation_name ASC
		LIMIT @limit`
	args := append(detailArgs(teamID, startMs, endMs, serviceName),
		clickhouse.Named("limit", limit),
		clickhouse.Named("cursorCount", cursor.TotalCount),
		clickhouse.Named("cursorOp", cursor.OperationName),
	)
	var rows []topDBQueryRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetTopDBQueriesCombined",
		&rows, query, args...); err != nil {
		return nil, err
	}
	for i := range rows {
		if len(rows[i].QS) >= 3 {
			rows[i].P50Ms = float32(rows[i].QS[0])
			rows[i].P95Ms = float32(rows[i].QS[1])
			rows[i].P99Ms = float32(rows[i].QS[2])
		}
	}
	return rows, nil
}

func serviceWherePred(serviceName string) string {
	if serviceName == "" {
		return ""
	}
	return "AND s.service = @serviceName"
}

func detailArgs(teamID int64, startMs, endMs int64, serviceName string) []any {
	args := chargs.RollupRangeArgs(teamID, startMs, endMs)
	if serviceName != "" {
		args = append(args, clickhouse.Named("serviceName", serviceName))
	}
	return args
}

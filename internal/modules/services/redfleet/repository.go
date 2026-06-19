package redfleet

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/mathutil"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetFleetREDMetrics(ctx context.Context, teamID int64, startMs, endMs int64) ([]redMetricsRow, error) {
	query := `
		SELECT service                                                                               AS service,
		       sumIf(value, metric_name = 'calls')                                                   AS total_count,
		       sumIf(value, metric_name = 'calls' AND JSONExtractString(attributes, 'status.code') = 'STATUS_CODE_ERROR') AS error_count,
		       sumMap(hist_buckets, hist_counts)                                                     AS hist
		FROM optikk.metrics
		PREWHERE team_id   = @teamID
		     AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		     AND timestamp BETWEEN @start AND @end
		     AND metric_name IN ('calls', 'duration')
		GROUP BY service`
	var rows []redMetricsRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetFleetREDMetrics",
		&rows, query, chargs.RollupRangeArgs(teamID, startMs, endMs)...); err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].QS = mathutil.Quantiles([]float64{0.5, 0.95, 0.99}, rows[i].HistTuple)
		if len(rows[i].QS) >= 3 {
			rows[i].P50Ms = rows[i].QS[0]
			rows[i].P95Ms = rows[i].QS[1]
			rows[i].P99Ms = rows[i].QS[2]
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
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       sumIf(value, metric_name = 'calls')    AS request_count,
		       sumIf(value, metric_name = 'calls' AND JSONExtractString(attributes, 'status.code') = 'STATUS_CODE_ERROR') AS error_count
		FROM optikk.metrics
		PREWHERE team_id     = @teamID
		     AND ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		     AND timestamp   BETWEEN @start AND @end
		     AND metric_name = 'calls'
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	var rows []requestRateRawRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetRequestAndErrorRateTimeSeries",
		&rows, query, chargs.RollupRangeArgs(teamID, startMs, endMs)...)
}

// statusBucketTimeseriesRow is one (bucket, status-class) row from spans_1m.
type statusBucketTimeseriesRow struct {
	BucketAt     time.Time `ch:"bucket_at"`
	StatusBucket string    `ch:"http_status_bucket"`
	RequestCount uint64    `ch:"request_count"`
}

// latencyPercentilesTimeseriesRow holds p50/p95/p99 for one display bucket.
type latencyPercentilesTimeseriesRow struct {
	BucketAt  time.Time `ch:"bucket_at"`
	HistTuple []any     `ch:"hist"`
	QS        []float32 `ch:"qs"`
	P50Ms     float32   `ch:"p50_ms"`
	P95Ms     float32   `ch:"p95_ms"`
	P99Ms     float32   `ch:"p99_ms"`
}

// topEndpointRow combines rate/error/percentile shape for one operation.
type topEndpointRow struct {
	ServiceName   string    `ch:"service"`
	OperationName string    `ch:"operation_name"`
	SpanKind      string    `ch:"kind_string"`
	HTTPRoute     string    `ch:"http_route"`
	TotalCount    uint64    `ch:"total_count"`
	ErrorCount    uint64    `ch:"error_count"`
	HistTuple     []any     `ch:"hist"`
	QS            []float32 `ch:"qs"`
	P50Ms         float32   `ch:"p50_ms"`
	P95Ms         float32   `ch:"p95_ms"`
	P99Ms         float32   `ch:"p99_ms"`
}

func (r *Repository) GetStatusTimeSeries(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string,
) ([]statusBucketTimeseriesRow, error) {
	grainSQL := timebucket.DisplayGrainSQL(endMs - startMs)
	query := `
		WITH active_fps AS (
		    SELECT DISTINCT fingerprint
		    FROM optikk.spans_resource
		    PREWHERE team_id   = @teamID
		         AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		         ` + serviceResourcePred(serviceName) + `
		)
		SELECT ` + grainSQL + ` AS bucket_at,
		       JSONExtractString(attributes, 'http.status_code') AS http_status_bucket,
		       sum(value) AS request_count
		FROM optikk.metrics
		PREWHERE team_id     = @teamID
		     AND ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		     AND fingerprint IN active_fps
		     AND timestamp   BETWEEN @start AND @end
		     AND metric_name = 'calls'
		WHERE 1=1
		      ` + serviceWherePred(serviceName) + `
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
		WITH active_fps AS (
		    SELECT DISTINCT fingerprint
		    FROM optikk.spans_resource
		    PREWHERE team_id   = @teamID
		         AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		         ` + serviceResourcePred(serviceName) + `
		)
		SELECT ` + grainSQL + ` AS bucket_at,
		       sumMap(hist_buckets, hist_counts) AS hist
		FROM optikk.metrics
		PREWHERE team_id     = @teamID
		     AND ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		     AND fingerprint IN active_fps
		     AND timestamp   BETWEEN @start AND @end
		     AND metric_name = 'duration'
		WHERE 1=1
		      ` + serviceWherePred(serviceName) + `
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	var rows []latencyPercentilesTimeseriesRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetLatencyPercentilesTimeSeries",
		&rows, query, detailArgs(teamID, startMs, endMs, serviceName)...); err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].QS = mathutil.Quantiles([]float64{0.5, 0.95, 0.99}, rows[i].HistTuple)
		if len(rows[i].QS) >= 3 {
			rows[i].P50Ms = rows[i].QS[0]
			rows[i].P95Ms = rows[i].QS[1]
			rows[i].P99Ms = rows[i].QS[2]
		}
	}
	return rows, nil
}

func (r *Repository) GetTopEndpointsCombined(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string, limit int, cursor TopEndpointsCursor,
) ([]topEndpointRow, error) {
	var paginationFilter string
	if !cursor.IsZero() {
		paginationFilter = "AND (total_count < @cursorCount OR (total_count = @cursorCount AND operation_name > @cursorOp))"
	}

	query := `
		WITH active_fps AS (
		    SELECT DISTINCT fingerprint
		    FROM optikk.spans_resource
		    PREWHERE team_id   = @teamID
		         AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		         ` + serviceResourcePred(serviceName) + `
		)
		SELECT service                                                                               AS service,
		       JSONExtractString(attributes, 'span.name')                                            AS operation_name,
		       any(JSONExtractString(attributes, 'span.kind'))                                       AS kind_string,
		       any(JSONExtractString(attributes, 'http.route'))                                      AS http_route,
		       sumIf(value, metric_name = 'calls')                                                   AS total_count,
		       sumIf(value, metric_name = 'calls' AND JSONExtractString(attributes, 'status.code') = 'STATUS_CODE_ERROR') AS error_count,
		       sumMap(hist_buckets, hist_counts)                                                     AS hist
		FROM optikk.metrics
		PREWHERE team_id     = @teamID
		     AND ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		     AND fingerprint IN active_fps
		     AND timestamp   BETWEEN @start AND @end
		     AND metric_name IN ('calls', 'duration')
		WHERE 1=1
		      ` + serviceWherePred(serviceName) + `
		GROUP BY service, operation_name
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
		rows[i].QS = mathutil.Quantiles([]float64{0.5, 0.95, 0.99}, rows[i].HistTuple)
		if len(rows[i].QS) >= 3 {
			rows[i].P50Ms = rows[i].QS[0]
			rows[i].P95Ms = rows[i].QS[1]
			rows[i].P99Ms = rows[i].QS[2]
		}
	}
	return rows, nil
}

func serviceResourcePred(serviceName string) string {
	if serviceName == "" {
		return ""
	}
	return "AND service = @serviceName"
}

func serviceWherePred(serviceName string) string {
	if serviceName == "" {
		return ""
	}
	return "AND service = @serviceName"
}

func detailArgs(teamID int64, startMs, endMs int64, serviceName string) []any {
	args := chargs.RollupRangeArgs(teamID, startMs, endMs)
	if serviceName != "" {
		args = append(args, clickhouse.Named("serviceName", serviceName))
	}
	return args
}

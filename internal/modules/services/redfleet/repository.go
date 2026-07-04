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

// extractQS unpacks the quantiles array into typed p50/p95/p99 fields.
func extractQS(qs []float64) (p50, p95, p99 float32) {
	if len(qs) >= 3 {
		return float32(qs[0]), float32(qs[1]), float32(qs[2])
	}
	return 0, 0, 0
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetFleetREDMetrics(ctx context.Context, f REDFilters) ([]redMetricsRow, error) {
	seriesWhere, args := BuildREDClauses(f)
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           service,
		           ` + seriesattr.StatusCode + ` AS status_code
		    FROM optikk.metrics_series AS s
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE 1=1
		          ` + seriesWhere + `
		          AND ` + seriesattr.ServerKindPred + `
		    GROUP BY fingerprint, service, status_code
		)
		SELECT series.service                                              AS service,
		       sum(m.hist_count)                                           AS total_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)     AS error_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state) AS qs
		FROM ` + timebucket.MetricsHistRollup(f.EndMs-f.StartMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		     AND m.fingerprint IN (SELECT fingerprint FROM series)
		GROUP BY service`
	var rows []redMetricsRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetFleetREDMetrics",
		&rows, query, args...); err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].P50Ms, rows[i].P95Ms, rows[i].P99Ms = extractQS(rows[i].QS)
	}
	return rows, nil
}

type requestRateRawRow struct {
	BucketAt     time.Time `ch:"bucket_at"`
	RequestCount uint64    `ch:"request_count"`
	ErrorCount   uint64    `ch:"error_count"`
}

func (r *Repository) GetRequestAndErrorRateTimeSeries(ctx context.Context, f REDFilters) ([]requestRateRawRow, error) {
	seriesWhere, args := BuildREDClauses(f)
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           ` + seriesattr.StatusCode + ` AS status_code
		    FROM optikk.metrics_series AS s
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE 1=1
		          ` + seriesWhere + `
		          AND ` + seriesattr.ServerKindPred + `
		    GROUP BY fingerprint, status_code
		)
		SELECT ` + timebucket.DisplayGrainSQL(f.EndMs-f.StartMs) + ` AS bucket_at,
		       sum(m.hist_count)                                       AS request_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `) AS error_count
		FROM ` + timebucket.MetricsHistRollup(f.EndMs-f.StartMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		     AND m.fingerprint IN (SELECT fingerprint FROM series)
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	var rows []requestRateRawRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetRequestAndErrorRateTimeSeries",
		&rows, query, args...)
}

// statusBucketTimeseriesRow is one (bucket, status-class) row from spanmetrics.
type statusBucketTimeseriesRow struct {
	BucketAt     time.Time `ch:"bucket_at"`
	StatusBucket string    `ch:"http_status_bucket"`
	RequestCount uint64    `ch:"request_count"`
}

type latencyPercentilesTimeseriesRow struct {
	BucketAt time.Time `ch:"bucket_at"`
	QS       []float64 `ch:"qs"`
	P50Ms    float32   `ch:"p50_ms"`
	P95Ms    float32   `ch:"p95_ms"`
	P99Ms    float32   `ch:"p99_ms"`
}

type endpointRateRow struct {
	BucketAt     time.Time `ch:"bucket_at"`
	HTTPRoute    string    `ch:"http_route"`
	RequestCount uint64    `ch:"request_count"`
	ErrorCount   uint64    `ch:"error_count"`
	QS           []float64 `ch:"qs"`
}

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

func (r *Repository) GetStatusTimeSeries(ctx context.Context, f REDFilters) ([]statusBucketTimeseriesRow, error) {
	seriesWhere, args := BuildREDClauses(f)
	grainSQL := timebucket.DisplayGrainSQL(f.EndMs - f.StartMs)
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           ` + seriesattr.HTTPStatusCode + ` AS http_status_code
		    FROM optikk.metrics_series AS s
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE 1=1
		          ` + seriesWhere + `
		          AND ` + seriesattr.ServerKindPred + `
		    GROUP BY fingerprint, http_status_code
		)
		SELECT ` + grainSQL + ` AS bucket_at,
		       series.http_status_code AS http_status_bucket,
		       sum(m.hist_count)       AS request_count
		FROM ` + timebucket.MetricsHistRollup(f.EndMs-f.StartMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		     AND m.fingerprint IN (SELECT fingerprint FROM series)
		GROUP BY bucket_at, http_status_bucket
		ORDER BY bucket_at ASC
		LIMIT 10000`
	var rows []statusBucketTimeseriesRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetStatusTimeSeries",
		&rows, query, args...)
}

func (r *Repository) GetLatencyPercentilesTimeSeries(ctx context.Context, f REDFilters) ([]latencyPercentilesTimeseriesRow, error) {
	seriesWhere, args := BuildREDClauses(f)
	grainSQL := timebucket.DisplayGrainSQL(f.EndMs - f.StartMs)
	query := `
		WITH series AS (
		    SELECT fingerprint
		    FROM optikk.metrics_series AS s
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE 1=1
		          ` + seriesWhere + `
		          AND ` + seriesattr.ServerKindPred + `
		    GROUP BY fingerprint
		)
		SELECT ` + grainSQL + ` AS bucket_at,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state) AS qs
		FROM ` + timebucket.MetricsHistRollup(f.EndMs-f.StartMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		     AND m.fingerprint IN (SELECT fingerprint FROM series)
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	var rows []latencyPercentilesTimeseriesRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetLatencyPercentilesTimeSeries",
		&rows, query, args...); err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].P50Ms, rows[i].P95Ms, rows[i].P99Ms = extractQS(rows[i].QS)
	}
	return rows, nil
}

func (r *Repository) GetREDByEndpointTimeSeries(ctx context.Context, f REDFilters) ([]endpointRateRow, error) {
	seriesWhere, args := BuildREDClauses(f)
	grainSQL := timebucket.DisplayGrainSQL(f.EndMs - f.StartMs)
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           ` + seriesattr.HTTPRoute + `  AS http_route,
		           ` + seriesattr.StatusCode + ` AS status_code
		    FROM optikk.metrics_series AS s
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE 1=1
		          ` + seriesWhere + `
		          AND ` + seriesattr.ServerKindPred + `
		    GROUP BY fingerprint, http_route, status_code
		)
		SELECT ` + grainSQL + ` AS bucket_at,
		       series.http_route                                                  AS http_route,
		       sum(m.hist_count)                                                   AS request_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)             AS error_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state) AS qs
		FROM ` + timebucket.MetricsHistRollup(f.EndMs-f.StartMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		     AND m.fingerprint IN (SELECT fingerprint FROM series)
		GROUP BY bucket_at, http_route
		ORDER BY bucket_at ASC
		LIMIT 10000`
	var rows []endpointRateRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetREDByEndpointTimeSeries",
		&rows, query, args...)
}

func (r *Repository) GetTopEndpointsCombined(
	ctx context.Context, f REDFilters, limit int, cursor TopEndpointsCursor,
) ([]topEndpointRow, error) {
	seriesWhere, args := BuildREDClauses(f)
	var paginationFilter string
	if !cursor.IsZero() {
		paginationFilter = "AND (total_count < @cursorCount OR (total_count = @cursorCount AND operation_name > @cursorOp))"
	}

	query := `
		WITH series AS (
		    SELECT fingerprint,
		           service,
		           ` + seriesattr.SpanName + `   AS span_name,
		           ` + seriesattr.SpanKind + `   AS span_kind,
		           ` + seriesattr.HTTPRoute + `  AS http_route,
		           ` + seriesattr.StatusCode + ` AS status_code
		    FROM optikk.metrics_series AS s
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE 1=1
		          ` + seriesWhere + `
		          AND ` + seriesattr.ServerKindPred + `
		    GROUP BY fingerprint, service, span_name, span_kind, http_route, status_code
		)
		SELECT any(series.service)                                                  AS service,
		       series.span_name                                                     AS operation_name,
		       any(series.span_kind)                                                AS kind_string,
		       any(series.http_route)                                               AS http_route,
		       sum(m.hist_count)                                                    AS total_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)              AS error_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state)  AS qs
		FROM ` + timebucket.MetricsHistRollup(f.EndMs-f.StartMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		     AND m.fingerprint IN (SELECT fingerprint FROM series)
		GROUP BY operation_name
		HAVING operation_name != '' ` + paginationFilter + `
		ORDER BY total_count DESC, operation_name ASC
		LIMIT @limit`
	args = append(args,
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
		rows[i].P50Ms, rows[i].P95Ms, rows[i].P99Ms = extractQS(rows[i].QS)
	}
	return rows, nil
}

func (r *Repository) GetTopDBQueriesCombined(
	ctx context.Context, f REDFilters, limit int, cursor TopEndpointsCursor,
) ([]topDBQueryRow, error) {
	seriesWhere, args := BuildREDClauses(f)
	var paginationFilter string
	if !cursor.IsZero() {
		paginationFilter = "AND (total_count < @cursorCount OR (total_count = @cursorCount AND operation_name > @cursorOp))"
	}

	query := `
		WITH series AS (
		    SELECT fingerprint,
		           service,
		           ` + seriesattr.SpanName + `   AS span_name,
		           ` + seriesattr.DBSystem + `   AS db_system,
		           ` + seriesattr.StatusCode + ` AS status_code
		    FROM optikk.metrics_series AS s
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE 1=1
		          ` + seriesWhere + `
		          AND ` + seriesattr.DBSpanPred + `
		    GROUP BY fingerprint, service, span_name, db_system, status_code
		)
		SELECT any(series.service)                                                  AS service,
		       series.span_name                                                     AS operation_name,
		       any(series.db_system)                                                AS db_system,
		       sum(m.hist_count)                                                    AS total_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)              AS error_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state)  AS qs
		FROM ` + timebucket.MetricsHistRollup(f.EndMs-f.StartMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		     AND m.fingerprint IN (SELECT fingerprint FROM series)
		GROUP BY operation_name
		HAVING operation_name != '' ` + paginationFilter + `
		ORDER BY total_count DESC, operation_name ASC
		LIMIT @limit`
	args = append(args,
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
		rows[i].P50Ms, rows[i].P95Ms, rows[i].P99Ms = extractQS(rows[i].QS)
	}
	return rows, nil
}

type serviceRequestRateRawRow struct {
	BucketAt     time.Time `ch:"bucket_at"`
	ServiceName  string    `ch:"service_name"`
	RequestCount uint64    `ch:"request_count"`
}

func (r *Repository) GetRequestRateTimeSeries(ctx context.Context, f REDFilters) ([]serviceRequestRateRawRow, error) {
	seriesWhere, args := BuildREDClauses(f)
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           service AS service_name
		    FROM optikk.metrics_series AS s
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE 1=1
		          ` + seriesWhere + `
		          AND ` + seriesattr.ServerKindPred + `
		    GROUP BY fingerprint, service_name
		)
		SELECT ` + timebucket.DisplayGrainSQL(f.EndMs-f.StartMs) + ` AS bucket_at,
		       series.service_name                                     AS service_name,
		       sum(m.hist_count)                                       AS request_count
		FROM ` + timebucket.MetricsHistRollup(f.EndMs-f.StartMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		     AND m.fingerprint IN (SELECT fingerprint FROM series)
		GROUP BY bucket_at, service_name
		ORDER BY bucket_at ASC
		LIMIT 10000`
	var rows []serviceRequestRateRawRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetRequestRateTimeSeries",
		&rows, query, args...)
}

func (r *Repository) GetOperationBaseline(ctx context.Context, tenantID int64, startMs, endMs int64, serviceName, operationName string) (operationBaselineRow, error) {
	query := `
		WITH series AS (
		    SELECT fingerprint
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE ` + seriesattr.SpanName + ` = @operationName AND service = @serviceName
		    GROUP BY fingerprint
		)
		SELECT sum(m.hist_count)                                            AS span_count,
		       quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state) AS qs
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'`
	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("operationName", operationName),
	)
	var rows []operationBaselineRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetOperationBaseline",
		&rows, query, args...); err != nil {
		return operationBaselineRow{}, err
	}
	if len(rows) == 0 {
		return operationBaselineRow{}, nil
	}
	row := rows[0]
	if len(row.QS) >= 3 {
		row.P50Ms = float32(row.QS[0])
		row.P95Ms = float32(row.QS[1])
		row.P99Ms = float32(row.QS[2])
	}
	return row, nil
}

func (r *Repository) GetServiceSaturationAggs(
	ctx context.Context, tenantID int64, startMs, endMs int64, serviceName string, metricNames []string,
) ([]serviceMetricRow, error) {

	hostQuery := `
		SELECT DISTINCT host
		FROM optikk.metrics_series
		PREWHERE tenant_id     = @tenantID
		     AND metric_name = 'traces.span.metrics.duration'
		WHERE service = @serviceName
		  AND host    != ''`
	var hostRows []struct {
		Host string `ch:"host"`
	}
	hostArgs := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("serviceName", serviceName),
	}
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetServiceHosts",
		&hostRows, hostQuery, hostArgs...); err != nil {
		return nil, err
	}

	hosts := make([]string, len(hostRows))
	for i, row := range hostRows {
		hosts[i] = row.Host
	}

	query := `
		SELECT
		    @serviceName                      AS service,
		    m.metric_name                     AS metric_name,
		    sum(m.val_sum) / sum(m.val_count) AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN optikk.metrics_series AS s ON m.fingerprint = s.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		WHERE s.service = @serviceName
		   OR (s.host != '' AND s.host IN @hosts)
		GROUP BY metric_name`

	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("metricNames", metricNames),
		clickhouse.Named("hosts", hosts),
	)
	var rows []serviceMetricRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetServiceSaturationAggs",
		&rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) GetServiceSaturationTimeSeries(
	ctx context.Context, tenantID int64, startMs, endMs int64, serviceName string, metricNames []string,
) ([]saturationTimeSeriesRawRow, error) {

	hostQuery := `
		SELECT DISTINCT host
		FROM optikk.metrics_series
		PREWHERE tenant_id     = @tenantID
		     AND metric_name = 'traces.span.metrics.duration'
		WHERE service = @serviceName
		  AND host    != ''`
	var hostRows []struct {
		Host string `ch:"host"`
	}
	hostArgs := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("serviceName", serviceName),
	}
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetServiceHosts",
		&hostRows, hostQuery, hostArgs...); err != nil {
		return nil, err
	}

	hosts := make([]string, len(hostRows))
	for i, row := range hostRows {
		hosts[i] = row.Host
	}

	grainSQL := timebucket.DisplayGrainSQL(endMs - startMs)
	query := `
		SELECT
		    ` + grainSQL + ` AS bucket_at,
		    sum(m.val_sum) / sum(m.val_count) AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN optikk.metrics_series AS s ON m.fingerprint = s.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		WHERE s.service = @serviceName
		   OR (s.host != '' AND s.host IN @hosts)
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`

	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("metricNames", metricNames),
		clickhouse.Named("hosts", hosts),
	)
	var rows []saturationTimeSeriesRawRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetServiceSaturationTimeSeries",
		&rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

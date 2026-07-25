package redfleet

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type Repository struct {
	db clickhouse.Conn
}

// extractQS narrows the projected quantiles to this package's float32 fields.
func extractQS(qs []float64) (p50, p95, p99 float32) {
	a, b, c := spanstats.LatencyP50P95P99.P50P95P99(qs)
	return float32(a), float32(b), float32(c)
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetFleetREDMetrics(ctx context.Context, f REDFilters) ([]redMetricsRow, error) {
	where, args := BuildREDClauses(f)
	query := `
		SELECT service            AS service_name,
		       grouping(service)  AS is_total,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.LatencyP50P95P99.SQL() + `
		FROM ` + timebucket.SpanStatsRollup(f.EndMs-f.StartMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end` + where + `
		GROUP BY GROUPING SETS ((service_name), ())`
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
	RequestCount uint64    `ch:"request_total"`
	ErrorCount   uint64    `ch:"error_total"`
}

func (r *Repository) GetRequestAndErrorRateTimeSeries(ctx context.Context, f REDFilters) ([]requestRateRawRow, error) {
	where, args := BuildREDClauses(f)
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(f.EndMs-f.StartMs) + ` AS bucket_at,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `
		FROM ` + timebucket.SpanStatsRollup(f.EndMs-f.StartMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end` + where + `
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	var rows []requestRateRawRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetRequestAndErrorRateTimeSeries",
		&rows, query, args...)
}

// statusBucketTimeseriesRow is one (bucket, status-class) row from span_stats.
type statusBucketTimeseriesRow struct {
	BucketAt     time.Time `ch:"bucket_at"`
	StatusBucket string    `ch:"http_status_bucket"`
	RequestCount uint64    `ch:"request_total"`
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
	RequestCount uint64    `ch:"request_total"`
	ErrorCount   uint64    `ch:"error_total"`
	QS           []float64 `ch:"qs"`
}

type topDBQueryRow struct {
	ServiceName   string    `ch:"service_any"`
	OperationName string    `ch:"operation_name"`
	DBSystem      string    `ch:"db_system_any"`
	TotalCount    uint64    `ch:"request_total"`
	ErrorCount    uint64    `ch:"error_total"`
	QS            []float64 `ch:"qs"`
	P50Ms         float32   `ch:"p50_ms"`
	P95Ms         float32   `ch:"p95_ms"`
	P99Ms         float32   `ch:"p99_ms"`
}

type topEndpointRow struct {
	ServiceName   string    `ch:"service_any"`
	OperationName string    `ch:"operation_name"`
	SpanKind      string    `ch:"kind_string_any"`
	HTTPRoute     string    `ch:"http_route_any"`
	HTTPMethod    string    `ch:"http_method_any"`
	RPCSystem     string    `ch:"rpc_system_any"`
	TotalCount    uint64    `ch:"request_total"`
	ErrorCount    uint64    `ch:"error_total"`
	QS            []float64 `ch:"qs"`
	P50Ms         float32   `ch:"p50_ms"`
	P95Ms         float32   `ch:"p95_ms"`
	P99Ms         float32   `ch:"p99_ms"`
}

func (r *Repository) GetStatusTimeSeries(ctx context.Context, f REDFilters) ([]statusBucketTimeseriesRow, error) {
	where, args := BuildREDClauses(f)
	grainSQL := timebucket.DisplayGrainSQL(f.EndMs - f.StartMs)
	query := `
		SELECT ` + grainSQL + `    AS bucket_at,
		       http_status_bucket AS http_status_bucket,
		       ` + spanstats.Requests + `
		FROM ` + timebucket.SpanStatsRollup(f.EndMs-f.StartMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end` + where + `
		GROUP BY bucket_at, http_status_bucket
		ORDER BY bucket_at ASC
		LIMIT 10000`
	var rows []statusBucketTimeseriesRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetStatusTimeSeries",
		&rows, query, args...)
}

func (r *Repository) GetLatencyPercentilesTimeSeries(ctx context.Context, f REDFilters) ([]latencyPercentilesTimeseriesRow, error) {
	where, args := BuildREDClauses(f)
	grainSQL := timebucket.DisplayGrainSQL(f.EndMs - f.StartMs)
	query := `
		SELECT ` + grainSQL + ` AS bucket_at,
		       ` + spanstats.LatencyP50P95P99.SQL() + `
		FROM ` + timebucket.SpanStatsRollup(f.EndMs-f.StartMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end` + where + `
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
	where, args := BuildREDClauses(f)
	grainSQL := timebucket.DisplayGrainSQL(f.EndMs - f.StartMs)
	query := `
		SELECT ` + grainSQL + ` AS bucket_at,
		       http_route       AS http_route,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.LatencyP50P95P99.SQL() + `
		FROM ` + timebucket.SpanStatsRollup(f.EndMs-f.StartMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end` + where + `
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
	where, args := BuildREDClauses(f)
	var paginationFilter string
	if !cursor.IsZero() {
		paginationFilter = "AND (" + spanstats.RequestTotal + " < @cursorCount OR (" +
			spanstats.RequestTotal + " = @cursorCount AND operation_name > @cursorOp))"
	}

	query := `
		SELECT any(service)     AS service_any,
		       span_name        AS operation_name,
		       any(kind_string) AS kind_string_any,
		       any(http_route)  AS http_route_any,
		       any(http_method) AS http_method_any,
		       any(rpc_system)  AS rpc_system_any,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.LatencyP50P95P99.SQL() + `
		FROM ` + timebucket.SpanStatsRollup(f.EndMs-f.StartMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND span_name != ''` + where + `
		GROUP BY operation_name
		HAVING operation_name != '' ` + paginationFilter + `
		ORDER BY ` + spanstats.RequestTotal + ` DESC, operation_name ASC
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
	where, args := BuildREDClauses(f)
	var paginationFilter string
	if !cursor.IsZero() {
		paginationFilter = "AND (" + spanstats.RequestTotal + " < @cursorCount OR (" +
			spanstats.RequestTotal + " = @cursorCount AND operation_name > @cursorOp))"
	}

	query := `
		SELECT any(service)   AS service_any,
		       span_name      AS operation_name,
		       any(db_system) AS db_system_any,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.LatencyP50P95P99.SQL() + `
		FROM ` + timebucket.SpanStatsRollup(f.EndMs-f.StartMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND ` + spanstats.DBSpanPred + `
		     AND span_name != ''` + where + `
		GROUP BY operation_name
		HAVING operation_name != '' ` + paginationFilter + `
		ORDER BY ` + spanstats.RequestTotal + ` DESC, operation_name ASC
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
	RequestCount uint64    `ch:"request_total"`
}

func (r *Repository) GetRequestRateTimeSeries(ctx context.Context, f REDFilters) ([]serviceRequestRateRawRow, error) {
	where, args := BuildREDClauses(f)
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(f.EndMs-f.StartMs) + ` AS bucket_at,
		       service            AS service_name,
		       ` + spanstats.Requests + `
		FROM ` + timebucket.SpanStatsRollup(f.EndMs-f.StartMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end` + where + `
		GROUP BY bucket_at, service_name
		ORDER BY bucket_at ASC
		LIMIT 10000`
	var rows []serviceRequestRateRawRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetRequestRateTimeSeries",
		&rows, query, args...)
}

func (r *Repository) GetOperationBaseline(ctx context.Context, tenantID int64, startMs, endMs int64, serviceName, operationName string) (operationBaselineRow, error) {
	query := `
		SELECT ` + spanstats.Requests + `,
		       ` + spanstats.LatencyP50P95P99.SQL() + `
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND service   = @serviceName
		     AND span_name = @operationName`
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
	row.P50Ms, row.P95Ms, row.P99Ms = extractQS(row.QS)
	return row, nil
}

func (r *Repository) GetServiceSaturationAggs(
	ctx context.Context, tenantID int64, startMs, endMs int64, serviceName string, metricNames []string,
) ([]serviceMetricRow, error) {
	// service_hosts maps the service to its hosts via span_stats; the outer
	// join against metrics_series stays, since saturation metrics are genuine
	// OTel system metrics identified by fingerprint.
	query := `
		WITH service_hosts AS (
		    SELECT DISTINCT host
		    FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		      AND service = @serviceName AND host != ''
		)
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
		   OR (s.host != '' AND s.host IN (SELECT host FROM service_hosts))
		GROUP BY metric_name`

	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("metricNames", metricNames),
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
	grainSQL := timebucket.DisplayGrainSQL(endMs - startMs)
	query := `
		WITH service_hosts AS (
		    SELECT DISTINCT host
		    FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		      AND service = @serviceName AND host != ''
		)
		SELECT
		    ` + grainSQL + ` AS bucket_at,
		    sum(m.val_sum) / sum(m.val_count) AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN optikk.metrics_series AS s ON m.fingerprint = s.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		WHERE s.service = @serviceName
		   OR (s.host != '' AND s.host IN (SELECT host FROM service_hosts))
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`

	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("metricNames", metricNames),
	)
	var rows []saturationTimeSeriesRawRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetServiceSaturationTimeSeries",
		&rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

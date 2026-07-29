package redfleet

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type Repository struct {
	db clickhouse.Conn
}

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

type statusBucketTimeseriesRow struct {
	BucketAt    time.Time `ch:"bucket_at"`
	Status2xx   uint64    `ch:"s2xx"`
	Status4xx   uint64    `ch:"s4xx"`
	Status5xx   uint64    `ch:"s5xx"`
	StatusOther uint64    `ch:"s_other"`
}

type latencyPercentilesTimeseriesRow struct {
	BucketAt time.Time `ch:"bucket_at"`
	QS       []float64 `ch:"qs"`
	P50Ms    float32   `ch:"p50_ms"`
	P95Ms    float32   `ch:"p95_ms"`
	P99Ms    float32   `ch:"p99_ms"`
}

type endpointRateRow struct {
	BucketAt      time.Time `ch:"bucket_at"`
	OperationName string    `ch:"operation_name"`
	RequestCount  uint64    `ch:"request_total"`
	ErrorCount    uint64    `ch:"error_total"`
	QS            []float64 `ch:"qs"`
}

// Fallback series count when the caller does not ask for one. Callers pass an
// explicit `limit`; the hard cap is httputil.MaxPageSize, applied at the handler.
const defaultEndpointSeriesLimit = 20

func (r *Repository) GetStatusTimeSeries(ctx context.Context, f REDFilters) ([]statusBucketTimeseriesRow, error) {
	where, args := BuildREDClauses(f)
	grainSQL := timebucket.DisplayGrainSQL(f.EndMs - f.StartMs)
	query := `
		SELECT ` + grainSQL + ` AS bucket_at,
		       sumIf(request_count, http_status_bucket = '2xx') AS s2xx,
		       sumIf(request_count, http_status_bucket = '4xx') AS s4xx,
		       sumIf(request_count, http_status_bucket = '5xx') AS s5xx,
		       sumIf(request_count, http_status_bucket NOT IN ('2xx', '4xx', '5xx')) AS s_other
		FROM ` + timebucket.SpanStatsRollup(f.EndMs-f.StartMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end` + where + `
		GROUP BY bucket_at
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

func (r *Repository) GetREDByEndpointTimeSeries(ctx context.Context, f REDFilters, limit int) ([]endpointRateRow, error) {
	query, args := buildREDByEndpointQuery(f, limit)
	var rows []endpointRateRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetREDByEndpointTimeSeries",
		&rows, query, args...)
}

// Split out from the repository method so the generated SQL can be exercised
// without a database.
func buildREDByEndpointQuery(f REDFilters, limit int) (string, []any) {
	if limit <= 0 {
		limit = defaultEndpointSeriesLimit
	}
	where, args := BuildREDClauses(f)
	grainSQL := timebucket.DisplayGrainSQL(f.EndMs - f.StartMs)
	rollup := timebucket.SpanStatsRollup(f.EndMs - f.StartMs)
	// Grouped by span_name like top-endpoints: http_route is empty on gRPC and
	// on clients that never learned a route, which collapsed every series.
	query := `
		WITH top_operations AS (
		    SELECT span_name
		    FROM ` + rollup + `
		    PREWHERE tenant_id = @tenantID
		         AND timestamp BETWEEN @start AND @end
		         AND span_name != ''` + where + `
		    GROUP BY span_name
		    ORDER BY sum(request_count) DESC, span_name ASC
		    LIMIT @endpointLimit
		)
		SELECT ` + grainSQL + ` AS bucket_at,
		       span_name        AS operation_name,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.LatencyP99.SQL() + `
		FROM ` + rollup + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end` + where + `
		WHERE span_name IN (SELECT span_name FROM top_operations)
		GROUP BY bucket_at, operation_name
		ORDER BY bucket_at ASC
		LIMIT @rowLimit`
	// One row per (endpoint, bucket). A fixed cap would silently truncate the
	// tail of the window once the caller asks for many endpoints, so the cap
	// tracks the shape of the result the caller actually requested.
	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	buckets := int64(1)
	if grain > 0 {
		buckets += (f.EndMs - f.StartMs) / grain.Milliseconds()
	}
	args = append(args,
		clickhouse.Named("endpointLimit", limit),
		clickhouse.Named("rowLimit", int64(limit)*(buckets+1)),
	)
	return query, args
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

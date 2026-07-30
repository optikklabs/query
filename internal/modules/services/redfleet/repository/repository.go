package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/services/redfleet/filter"
	"github.com/optikklabs/query/internal/modules/services/redfleet/models"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func extractQS(qs []float64) (p50, p95, p99 float32) {
	a, b, c := spanstats.LatencyP50P95P99.P50P95P99(qs)
	return float32(a), float32(b), float32(c)
}

func (r *Repository) GetFleetREDMetrics(ctx context.Context, f filter.Filters) ([]models.REDMetricsRow, error) {
	where, args := filter.BuildClauses(f)
	query := `
		SELECT service            AS service_name,
		       grouping(service)  AS is_total,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.LatencyP50P95P99.SQL() + `
		FROM ` + timebucket.SpanStatsRollup(f.StartMs, f.EndMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end` + where + `
		GROUP BY GROUPING SETS ((service_name), ())`
	var rows []models.REDMetricsRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetFleetREDMetrics",
		&rows, query, args...); err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].P50Ms, rows[i].P95Ms, rows[i].P99Ms = extractQS(rows[i].QS)
	}
	return rows, nil
}

func (r *Repository) GetRequestAndErrorRateTimeSeries(ctx context.Context, f filter.Filters) ([]models.RequestRateRawRow, error) {
	where, args := filter.BuildClauses(f)
	query := `
		SELECT ` + timebucket.DisplayGrainSQLForRange(f.StartMs, f.EndMs) + ` AS bucket_at,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `
		FROM ` + timebucket.SpanStatsRollup(f.StartMs, f.EndMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end` + where + `
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	var rows []models.RequestRateRawRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetRequestAndErrorRateTimeSeries",
		&rows, query, args...)
}

func (r *Repository) GetStatusTimeSeries(ctx context.Context, f filter.Filters) ([]models.StatusBucketRow, error) {
	where, args := filter.BuildClauses(f)
	grainSQL := timebucket.DisplayGrainSQLForRange(f.StartMs, f.EndMs)
	query := `
		SELECT ` + grainSQL + ` AS bucket_at,
		       sumIf(request_count, http_status_bucket = '2xx') AS s2xx,
		       sumIf(request_count, http_status_bucket = '4xx') AS s4xx,
		       sumIf(request_count, http_status_bucket = '5xx') AS s5xx,
		       sumIf(request_count, http_status_bucket NOT IN ('2xx', '4xx', '5xx')) AS s_other
		FROM ` + timebucket.SpanStatsRollup(f.StartMs, f.EndMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end` + where + `
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	var rows []models.StatusBucketRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetStatusTimeSeries",
		&rows, query, args...)
}

func (r *Repository) GetLatencyPercentilesTimeSeries(ctx context.Context, f filter.Filters) ([]models.LatencyPercentilesRow, error) {
	where, args := filter.BuildClauses(f)
	grainSQL := timebucket.DisplayGrainSQLForRange(f.StartMs, f.EndMs)
	query := `
		SELECT ` + grainSQL + ` AS bucket_at,
		       ` + spanstats.LatencyP50P95P99.SQL() + `
		FROM ` + timebucket.SpanStatsRollup(f.StartMs, f.EndMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end` + where + `
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	var rows []models.LatencyPercentilesRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetLatencyPercentilesTimeSeries",
		&rows, query, args...); err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].P50Ms, rows[i].P95Ms, rows[i].P99Ms = extractQS(rows[i].QS)
	}
	return rows, nil
}

func (r *Repository) GetREDByEndpointTimeSeries(ctx context.Context, f filter.Filters) ([]models.EndpointRateRow, error) {
	query, args := buildREDByEndpointQuery(f)
	var rows []models.EndpointRateRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetREDByEndpointTimeSeries",
		&rows, query, args...)
}

// Split out so the generated SQL can be exercised without a database.
func buildREDByEndpointQuery(f filter.Filters) (string, []any) {
	where, args := filter.BuildClauses(f)
	grainSQL := timebucket.DisplayGrainSQLForRange(f.StartMs, f.EndMs)
	rollup := timebucket.SpanStatsRollup(f.StartMs, f.EndMs)
	query := `
		SELECT ` + grainSQL + ` AS bucket_at,
		       span_name        AS operation_name,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.LatencyP99.SQL() + `
		FROM ` + rollup + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end` + where + `
		GROUP BY bucket_at, operation_name
		ORDER BY bucket_at ASC, operation_name ASC`
	return query, args
}

func (r *Repository) GetRequestRateTimeSeries(ctx context.Context, f filter.Filters) ([]models.ServiceRequestRateRow, error) {
	where, args := filter.BuildClauses(f)
	query := `
		SELECT ` + timebucket.DisplayGrainSQLForRange(f.StartMs, f.EndMs) + ` AS bucket_at,
		       service            AS service_name,
		       ` + spanstats.Requests + `
		FROM ` + timebucket.SpanStatsRollup(f.StartMs, f.EndMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end` + where + `
		GROUP BY bucket_at, service_name
		ORDER BY bucket_at ASC`
	var rows []models.ServiceRequestRateRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetRequestRateTimeSeries",
		&rows, query, args...)
}

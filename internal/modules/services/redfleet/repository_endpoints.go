package redfleet

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

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
		       anyIf(http_method, http_method != '') AS http_method_any,
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
	// Database calls are CLIENT spans, so this one skips the inbound filter.
	where, args := buildServiceClauses(f)
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

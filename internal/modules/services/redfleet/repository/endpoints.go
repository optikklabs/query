package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/services/redfleet/filter"
	"github.com/optikklabs/query/internal/modules/services/redfleet/models"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

func endpointCursorFilter(cursor models.TopEndpointsCursor) string {
	if cursor.IsZero() {
		return ""
	}
	return "AND (" + spanstats.RequestTotal + " < @cursorCount OR (" +
		spanstats.RequestTotal + " = @cursorCount AND " +
		"(service_any, operation_name, kind_string_any, http_route_any, http_method_any, rpc_system_any) > " +
		"(@cursorService, @cursorOp, @cursorKind, @cursorRoute, @cursorMethod, @cursorRPC)))"
}

func dbCursorFilter(cursor models.TopEndpointsCursor) string {
	if cursor.IsZero() {
		return ""
	}
	return "AND (" + spanstats.RequestTotal + " < @cursorCount OR (" +
		spanstats.RequestTotal + " = @cursorCount AND (service_any, operation_name, db_system_any) > " +
		"(@cursorService, @cursorOp, @cursorDB)))"
}

func cursorArgs(args []any, limit int, cursor models.TopEndpointsCursor) []any {
	return append(args,
		clickhouse.Named("limit", limit),
		clickhouse.Named("cursorCount", cursor.TotalCount),
		clickhouse.Named("cursorService", cursor.ServiceName),
		clickhouse.Named("cursorOp", cursor.OperationName),
		clickhouse.Named("cursorKind", cursor.SpanKind),
		clickhouse.Named("cursorRoute", cursor.HTTPRoute),
		clickhouse.Named("cursorMethod", cursor.HTTPMethod),
		clickhouse.Named("cursorRPC", cursor.RPCSystem),
		clickhouse.Named("cursorDB", cursor.DBSystem),
	)
}

func (r *Repository) GetTopEndpointsCombined(
	ctx context.Context, f filter.Filters, limit int, cursor models.TopEndpointsCursor,
) ([]models.TopEndpointRow, error) {
	where, args := filter.BuildClauses(f)
	query := `
		SELECT service      AS service_any,
		       span_name   AS operation_name,
		       kind_string AS kind_string_any,
		       http_route  AS http_route_any,
		       http_method AS http_method_any,
		       rpc_system  AS rpc_system_any,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.LatencyP50P95P99.SQL() + `
		FROM ` + timebucket.SpanStatsRollup(f.StartMs, f.EndMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND span_name != ''` + where + `
		GROUP BY service_any, operation_name, kind_string_any, http_route_any, http_method_any, rpc_system_any
		HAVING operation_name != '' ` + endpointCursorFilter(cursor) + `
		ORDER BY ` + spanstats.RequestTotal + ` DESC,
		         service_any, operation_name, kind_string_any, http_route_any, http_method_any, rpc_system_any
		LIMIT @limit`
	var rows []models.TopEndpointRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetTopEndpointsCombined",
		&rows, query, cursorArgs(args, limit, cursor)...); err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].P50Ms, rows[i].P95Ms, rows[i].P99Ms = extractQS(rows[i].QS)
	}
	return rows, nil
}

func (r *Repository) GetTopDBQueriesCombined(
	ctx context.Context, f filter.Filters, limit int, cursor models.TopEndpointsCursor,
) ([]models.TopDBQueryRow, error) {
	// Database calls are CLIENT spans, so this one skips the inbound filter.
	where, args := filter.BuildServiceClauses(f)
	query := `
		SELECT service   AS service_any,
		       span_name AS operation_name,
		       db_system AS db_system_any,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.LatencyP50P95P99.SQL() + `
		FROM ` + timebucket.SpanStatsRollup(f.StartMs, f.EndMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND ` + spanstats.DBSpanPred + `
		     AND span_name != ''` + where + `
		GROUP BY service_any, operation_name, db_system_any
		HAVING operation_name != '' ` + dbCursorFilter(cursor) + `
		ORDER BY ` + spanstats.RequestTotal + ` DESC, service_any, operation_name, db_system_any
		LIMIT @limit`
	var rows []models.TopDBQueryRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetTopDBQueriesCombined",
		&rows, query, cursorArgs(args, limit, cursor)...); err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].P50Ms, rows[i].P95Ms, rows[i].P99Ms = extractQS(rows[i].QS)
	}
	return rows, nil
}

func (r *Repository) GetOperationBaseline(
	ctx context.Context, tenantID int64, startMs, endMs int64, serviceName, operationName string,
) (models.OperationBaselineRow, error) {
	query := `
		SELECT ` + spanstats.Requests + `,
		       ` + spanstats.LatencyP50P95P99.SQL() + `
		FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND service   = @serviceName
		     AND span_name = @operationName`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("operationName", operationName),
	)
	var rows []models.OperationBaselineRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetOperationBaseline",
		&rows, query, args...); err != nil {
		return models.OperationBaselineRow{}, err
	}
	if len(rows) == 0 {
		return models.OperationBaselineRow{}, nil
	}
	row := rows[0]
	row.P50Ms, row.P95Ms, row.P99Ms = extractQS(row.QS)
	return row, nil
}

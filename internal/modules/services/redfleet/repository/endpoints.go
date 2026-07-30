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

func cursorFilter(cursor models.TopEndpointsCursor) string {
	if cursor.IsZero() {
		return ""
	}
	return "AND (" + spanstats.RequestTotal + " < @cursorCount OR (" +
		spanstats.RequestTotal + " = @cursorCount AND operation_name > @cursorOp))"
}

func cursorArgs(args []any, limit int, cursor models.TopEndpointsCursor) []any {
	return append(args,
		clickhouse.Named("limit", limit),
		clickhouse.Named("cursorCount", cursor.TotalCount),
		clickhouse.Named("cursorOp", cursor.OperationName),
	)
}

func (r *Repository) GetTopEndpointsCombined(
	ctx context.Context, f filter.Filters, limit int, cursor models.TopEndpointsCursor,
) ([]models.TopEndpointRow, error) {
	where, args := filter.BuildClauses(f)
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
		HAVING operation_name != '' ` + cursorFilter(cursor) + `
		ORDER BY ` + spanstats.RequestTotal + ` DESC, operation_name ASC
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
		HAVING operation_name != '' ` + cursorFilter(cursor) + `
		ORDER BY ` + spanstats.RequestTotal + ` DESC, operation_name ASC
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
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND service   = @serviceName
		     AND span_name = @operationName`
	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
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

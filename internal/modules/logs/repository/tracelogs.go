package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/logs/models"
)

func (r *Repository) FetchLogsByTrace(ctx context.Context, tenantID int64, traceID string, limit int) ([]models.LogRow, error) {
	const query = `
		SELECT ` + models.LogColumns + `
		FROM optikk.logs
		PREWHERE tenant_id = @tenantID
		     AND trace_id = @traceID
		ORDER BY timestamp ASC
		LIMIT @limit`

	args := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("traceID", traceID),
		clickhouse.Named("limit", uint64(limit)),
	}

	var rows []models.LogRow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "logsTraceLogs.FetchLogsByTrace", &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

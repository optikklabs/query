package repository

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/logs/models"
)

func (r *Repository) GetByID(ctx context.Context, tenantID int64, logID string, startMs, endMs int64) (*models.LogRow, error) {
	args := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("logID", logID),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("startBucket", uint32((startMs/1000)/300*300)),
		clickhouse.Named("endBucket", uint32((endMs/1000)/300*300)),
	}

	query := `
		SELECT ` + models.LogColumns + `
		FROM optikk.logs
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND ts_bucket BETWEEN @startBucket AND @endBucket
		     AND log_id = @logID
		LIMIT 1`

	var rows []models.LogRow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "logsDetail.GetByID", &rows, query, args...); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/logs/models"
	"github.com/optikklabs/query/internal/shared/chargs"
)

// GetByID returns the log, or sql.ErrNoRows.
func (r *Repository) GetByID(ctx context.Context, tenantID int64, logID string, startMs, endMs int64) (models.LogRow, error) {
	args := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("logID", logID),
		chargs.Millis("start", startMs),
		chargs.Millis("end", endMs),
		clickhouse.Named("startBucket", timebucket.LogBucket(startMs)),
		clickhouse.Named("endBucket", timebucket.LogBucket(endMs)),
	}

	query := `
		SELECT ` + models.LogColumns + `
		FROM optikk.logs
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND ts_bucket BETWEEN @startBucket AND @endBucket
		     AND log_id = @logID
		LIMIT 1`

	var row models.LogRow
	err := dbutil.QueryRowCH(dbutil.ExplorerCtx(ctx), r.db, "logsDetail.GetByID", &row, query, args...)
	return row, err
}

package trace_logs

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/logs/shared/models"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository { return &Repository{db: db} }

func (r *Repository) FetchLogsByTrace(ctx context.Context, teamID int64, traceID string, limit int) ([]models.LogRow, error) {
	const query = `
		WITH trace_loc AS (
		    SELECT timestamp
		    FROM optikk.trace_index
		    PREWHERE trace_id = @traceID AND team_id = @teamID
		    LIMIT 1
		)
		SELECT ` + models.LogColumns + `
		FROM optikk.logs
		PREWHERE team_id = @teamID
		     AND timestamp BETWEEN (SELECT timestamp FROM trace_loc) - INTERVAL 5 MINUTE
		                       AND (SELECT timestamp FROM trace_loc) + INTERVAL 24 HOUR
		     AND trace_id = @traceID
		ORDER BY timestamp ASC
		LIMIT @limit`

	args := []any{
		clickhouse.Named("teamID", uint32(teamID)),
		clickhouse.Named("traceID", traceID),
		clickhouse.Named("limit", uint64(limit)),
	}

	var rows []models.LogRow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "logsTraceLogs.FetchLogsByTrace", &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

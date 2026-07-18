package explorer

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/logs/filter"
	"github.com/optikklabs/query/internal/modules/logs/shared/models"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository { return &Repository{db: db} }

func (r *Repository) getLogs(ctx context.Context, f filter.Filters, limit int, cur models.Cursor) ([]models.LogRow, bool, error) {
	resourceWhere, where, args := filter.BuildClauses(f)
	if !cur.IsZero() {
		where += ` AND (timestamp, log_id) < (@curTs, @curLid)`
		args = append(args,
			// DateNamed with ns scale; a plain time.Time arg truncates to seconds.
			clickhouse.DateNamed("curTs", cur.Timestamp, clickhouse.NanoSeconds),
			clickhouse.Named("curLid", cur.LogID),
		)
	}
	args = append(args, clickhouse.Named("pgLimit", uint64(limit+1)))

	cte, prewhereFP := filter.BuildFingerprintCTE(resourceWhere)
	query := cte + `
		SELECT ` + models.LogColumns + `
		FROM optikk.logs
		PREWHERE tenant_id = @tenantID` + prewhereFP + `
		     AND timestamp BETWEEN @start AND @end` + where + `
		WHERE timestamp BETWEEN @start AND @end
		ORDER BY timestamp DESC, log_id DESC
		LIMIT @pgLimit`

	var rows []models.LogRow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "logs.ListLogs", &rows, query, args...); err != nil {
		return nil, false, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	return rows, hasMore, nil
}

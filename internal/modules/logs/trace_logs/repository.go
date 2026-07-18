package trace_logs

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/logs/shared/models"
	"github.com/optikklabs/query/internal/shared/tracewindow"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository { return &Repository{db: db} }

// ResolveWindow locates the trace so the log read below can be bounded.
func (r *Repository) ResolveWindow(ctx context.Context, tenantID int64, traceID string) (tracewindow.Window, bool, error) {
	return tracewindow.Resolve(ctx, r.db, tenantID, traceID)
}

func (r *Repository) FetchLogsByTrace(ctx context.Context, tenantID int64, traceID string, limit int, w tracewindow.Window) ([]models.LogRow, error) {
	const query = `
		SELECT ` + models.LogColumns + `
		FROM optikk.logs
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id = @traceID
		ORDER BY timestamp ASC
		LIMIT @limit`

	args := append(tracewindow.Args(tenantID, traceID, w), clickhouse.Named("limit", uint64(limit)))

	var rows []models.LogRow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "logsTraceLogs.FetchLogsByTrace", &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

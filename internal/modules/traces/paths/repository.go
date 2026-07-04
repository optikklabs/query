package paths

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetCriticalPath(ctx context.Context, tenantID int64, traceID string) ([]criticalPathRow, error) {
	const query = `
		WITH trace_loc AS (
		    SELECT timestamp
		    FROM optikk.trace_index
		    PREWHERE trace_id = @traceID AND tenant_id = @tenantID
		    LIMIT 1
		)
		SELECT span_id,
		       parent_span_id,
		       name                       AS operation_name,
		       service,
		       duration_nano / 1000000.0  AS duration_ms,
		       timestamp,
		       duration_nano
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN (SELECT timestamp FROM trace_loc) - INTERVAL 5 MINUTE
		                       AND (SELECT timestamp FROM trace_loc) + INTERVAL 24 HOUR
		     AND trace_id = @traceID
		ORDER BY timestamp ASC
		LIMIT 5000`
	var rows []criticalPathRow
	return rows, dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "paths.GetCriticalPath", &rows, query, traceIDArgs(tenantID, traceID)...)
}

func (r *Repository) GetErrorPath(ctx context.Context, tenantID int64, traceID string) ([]errorPathRow, error) {
	const query = `
		WITH trace_loc AS (
		    SELECT timestamp
		    FROM optikk.trace_index
		    PREWHERE trace_id = @traceID AND tenant_id = @tenantID
		    LIMIT 1
		)
		SELECT span_id,
		       parent_span_id,
		       name                       AS operation_name,
		       service                    AS service,
		       status_code_string         AS status,
		       status_message,
		       timestamp                  AS start_time,
		       duration_nano / 1000000.0  AS duration_ms
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN (SELECT timestamp FROM trace_loc) - INTERVAL 5 MINUTE
		                       AND (SELECT timestamp FROM trace_loc) + INTERVAL 24 HOUR
		     AND trace_id = @traceID
		WHERE has_error = true OR status_code_string = 'ERROR'
		ORDER BY timestamp ASC
		LIMIT 1000`
	var rows []errorPathRow
	return rows, dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "paths.GetErrorPath", &rows, query, traceIDArgs(tenantID, traceID)...)
}

func traceIDArgs(tenantID int64, traceID string) []any {
	return []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("traceID", traceID),
	}
}

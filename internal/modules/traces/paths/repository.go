package paths

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func traceArgs(tenantID int64, traceID string, startMs, endMs int64) []any {
	return []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("traceID", traceID),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
	}
}

func (r *Repository) GetCriticalPath(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) ([]criticalPathRow, error) {
	const query = `
		SELECT span_id,
		       parent_span_id,
		       name                       AS operation_name,
		       service,
		       duration_nano / 1000000.0  AS duration_ms,
		       timestamp,
		       duration_nano
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id = @traceID
		ORDER BY timestamp ASC
		LIMIT 5000`
	var rows []criticalPathRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "paths.GetCriticalPath", &rows, query,
		traceArgs(tenantID, traceID, startMs, endMs)...,
	)
	return rows, err
}

func (r *Repository) GetErrorPath(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) ([]errorPathRow, error) {
	const query = `
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
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id = @traceID
		WHERE is_error = 1
		ORDER BY timestamp ASC
		LIMIT 1000`
	var rows []errorPathRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "paths.GetErrorPath", &rows, query,
		traceArgs(tenantID, traceID, startMs, endMs)...,
	)
	return rows, err
}

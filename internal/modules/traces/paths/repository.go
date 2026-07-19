package paths

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/shared/chargs"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetCriticalPath(ctx context.Context, tenantID int64, traceID string, startTimeMs, endTimeMs int64) ([]criticalPathRow, error) {
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
	args := append(chargs.RangeArgs(tenantID, startTimeMs, endTimeMs), clickhouse.Named("traceID", traceID))
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "paths.GetCriticalPath", &rows, query,
		args...,
	)
	return rows, err
}

func (r *Repository) GetErrorPath(ctx context.Context, tenantID int64, traceID string, startTimeMs, endTimeMs int64) ([]errorPathRow, error) {
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
		WHERE has_error = true OR status_code_string = 'ERROR'
		ORDER BY timestamp ASC
		LIMIT 1000`
	var rows []errorPathRow
	args := append(chargs.RangeArgs(tenantID, startTimeMs, endTimeMs), clickhouse.Named("traceID", traceID))
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "paths.GetErrorPath", &rows, query,
		args...,
	)
	return rows, err
}

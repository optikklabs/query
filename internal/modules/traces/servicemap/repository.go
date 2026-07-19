package servicemap

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

func (r *Repository) GetServiceMapSpans(ctx context.Context, tenantID int64, traceID string, startTimeMs, endTimeMs int64) ([]serviceMapSpanRow, error) {
	const query = `
		SELECT span_id,
		       parent_span_id,
		       service,
		       duration_nano / 1000000.0 AS duration_ms,
		       has_error
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id = @traceID
		ORDER BY timestamp ASC
		LIMIT 10000`
	var rows []serviceMapSpanRow
	args := append(chargs.RangeArgs(tenantID, startTimeMs, endTimeMs), clickhouse.Named("traceID", traceID))
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "servicemap.GetServiceMapSpans", &rows, query,
		args...,
	)
	return rows, err
}

func (r *Repository) GetTraceErrors(ctx context.Context, tenantID int64, traceID string, startTimeMs, endTimeMs int64) ([]traceErrorRow, error) {
	const query = `
		SELECT span_id,
		       service,
		       name                       AS operation_name,
		       exception_type,
		       exception_message,
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
	var rows []traceErrorRow
	args := append(chargs.RangeArgs(tenantID, startTimeMs, endTimeMs), clickhouse.Named("traceID", traceID))
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "servicemap.GetTraceErrors", &rows, query,
		args...,
	)
	return rows, err
}

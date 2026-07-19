package servicemap

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

// traceArgs binds the identity of a trace. The idx_trace_id bloom filter keeps
// the lookup cheap without a time window.
func traceArgs(tenantID int64, traceID string) []any {
	return []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("traceID", traceID),
	}
}

func (r *Repository) GetServiceMapSpans(ctx context.Context, tenantID int64, traceID string) ([]serviceMapSpanRow, error) {
	const query = `
		SELECT span_id,
		       parent_span_id,
		       service,
		       duration_nano / 1000000.0 AS duration_ms,
		       is_error = 1              AS has_error
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND trace_id = @traceID
		ORDER BY timestamp ASC
		LIMIT 10000`
	var rows []serviceMapSpanRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "servicemap.GetServiceMapSpans", &rows, query,
		traceArgs(tenantID, traceID)...,
	)
	return rows, err
}

func (r *Repository) GetTraceErrors(ctx context.Context, tenantID int64, traceID string) ([]traceErrorRow, error) {
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
		     AND trace_id = @traceID
		WHERE is_error = 1
		ORDER BY timestamp ASC
		LIMIT 1000`
	var rows []traceErrorRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "servicemap.GetTraceErrors", &rows, query,
		traceArgs(tenantID, traceID)...,
	)
	return rows, err
}

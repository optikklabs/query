package servicemap

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/shared/tracewindow"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// ResolveWindow locates the trace, bounding every span read below.
func (r *Repository) ResolveWindow(ctx context.Context, tenantID int64, traceID string) (tracewindow.Window, bool, error) {
	return tracewindow.Resolve(ctx, r.db, tenantID, traceID)
}

func (r *Repository) GetServiceMapSpans(ctx context.Context, tenantID int64, traceID string, w tracewindow.Window) ([]serviceMapSpanRow, error) {
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
	return rows, dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "servicemap.GetServiceMapSpans", &rows, query,
		tracewindow.Args(tenantID, traceID, w)...,
	)
}

func (r *Repository) GetTraceErrors(ctx context.Context, tenantID int64, traceID string, w tracewindow.Window) ([]traceErrorRow, error) {
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
	return rows, dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "servicemap.GetTraceErrors", &rows, query,
		tracewindow.Args(tenantID, traceID, w)...,
	)
}

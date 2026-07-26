package repository

import (
	"context"
	"time"

	dbutil "github.com/optikklabs/query/internal/infra/database"
)

// ServiceMapSpanRow is the minimum needed to derive the per-trace graph: the
// parent link identifies edges, the service names the nodes.
type ServiceMapSpanRow struct {
	SpanID       string  `ch:"span_id"`
	ParentSpanID string  `ch:"parent_span_id"`
	ServiceName  string  `ch:"service"`
	DurationMs   float64 `ch:"duration_ms"`
	HasError     bool    `ch:"has_error"`
}

// TraceErrorRow is an errored span, grouped by exception type in the service.
type TraceErrorRow struct {
	SpanID           string    `ch:"span_id"`
	ServiceName      string    `ch:"service"`
	OperationName    string    `ch:"operation_name"`
	ExceptionType    string    `ch:"exception_type"`
	ExceptionMessage string    `ch:"exception_message"`
	StatusMessage    string    `ch:"status_message"`
	StartTime        time.Time `ch:"start_time"`
	DurationMs       float64   `ch:"duration_ms"`
}

func (r *Repository) GetServiceMapSpans(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) ([]ServiceMapSpanRow, error) {
	const query = `
		SELECT span_id,
		       parent_span_id,
		       service,
		       duration_nano / 1000000.0 AS duration_ms,
		       is_error = 1              AS has_error
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id = @traceID
		ORDER BY timestamp ASC
		LIMIT 10000`
	var rows []ServiceMapSpanRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "servicemap.GetServiceMapSpans", &rows, query,
		boundedTraceArgs(tenantID, traceID, startMs, endMs)...,
	)
	return rows, err
}

func (r *Repository) GetTraceErrors(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) ([]TraceErrorRow, error) {
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
		WHERE is_error = 1
		ORDER BY timestamp ASC
		LIMIT 1000`
	var rows []TraceErrorRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "servicemap.GetTraceErrors", &rows, query,
		boundedTraceArgs(tenantID, traceID, startMs, endMs)...,
	)
	return rows, err
}

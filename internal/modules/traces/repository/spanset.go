package repository

import (
	"context"
	"time"

	dbutil "github.com/optikklabs/query/internal/infra/database"
)

// TraceSpanRow is the single per-trace span fetch that all trace-detail
// views (span list, critical path, error path, service map, error groups)
// are derived from in the service layer.
type TraceSpanRow struct {
	SpanID           string    `ch:"span_id"`
	ParentSpanID     string    `ch:"parent_span_id"`
	TraceID          string    `ch:"trace_id"`
	ServiceName      string    `ch:"service"`
	OperationName    string    `ch:"name"`
	KindString       string    `ch:"kind_string"`
	StatusCode       string    `ch:"status_code_string"`
	StatusMessage    string    `ch:"status_message"`
	ExceptionType    string    `ch:"exception_type"`
	ExceptionMessage string    `ch:"exception_message"`
	HasError         bool      `ch:"has_error"`
	DurationNano     uint64    `ch:"duration_nano"`
	Timestamp        time.Time `ch:"timestamp"`
}

// DurationMs mirrors the duration_nano / 1000000.0 projection the
// per-endpoint SQL used to compute.
func (r *TraceSpanRow) DurationMs() float64 {
	return float64(r.DurationNano) / 1_000_000.0
}

// ListTraceSpanRows scans a trace's spans once.
func (r *Repository) ListTraceSpanRows(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) ([]TraceSpanRow, error) {
	const query = `
		SELECT span_id,
		       parent_span_id,
		       trace_id,
		       service,
		       name,
		       kind_string,
		       status_code_string,
		       status_message,
		       exception_type,
		       exception_message,
		       is_error = 1 AS has_error,
		       duration_nano,
		       timestamp
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND trace_id = @traceID
		ORDER BY timestamp ASC, span_id ASC`
	var rows []TraceSpanRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "detail.ListTraceSpanRows", &rows, query,
		boundedTraceArgs(tenantID, traceID, startMs, endMs)...,
	)
	return rows, err
}

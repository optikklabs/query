package repository

import (
	"context"
	"time"

	dbutil "github.com/optikklabs/query/internal/infra/database"
)

type CriticalPathRow struct {
	SpanID        string    `ch:"span_id"`
	ParentSpanID  string    `ch:"parent_span_id"`
	OperationName string    `ch:"operation_name"`
	ServiceName   string    `ch:"service"`
	DurationMs    float64   `ch:"duration_ms"`
	Timestamp     time.Time `ch:"timestamp"`
	DurationNano  uint64    `ch:"duration_nano"`
}

type ErrorPathRow struct {
	SpanID        string    `ch:"span_id"`
	ParentSpanID  string    `ch:"parent_span_id"`
	OperationName string    `ch:"operation_name"`
	ServiceName   string    `ch:"service"`
	Status        string    `ch:"status"`
	StatusMessage string    `ch:"status_message"`
	StartTime     time.Time `ch:"start_time"`
	DurationMs    float64   `ch:"duration_ms"`
}

func (r *Repository) GetCriticalPath(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) ([]CriticalPathRow, error) {
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
	var rows []CriticalPathRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "paths.GetCriticalPath", &rows, query,
		boundedTraceArgs(tenantID, traceID, startMs, endMs)...,
	)
	return rows, err
}

func (r *Repository) GetErrorPath(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) ([]ErrorPathRow, error) {
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
	var rows []ErrorPathRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "paths.GetErrorPath", &rows, query,
		boundedTraceArgs(tenantID, traceID, startMs, endMs)...,
	)
	return rows, err
}

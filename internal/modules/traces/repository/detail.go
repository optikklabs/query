package repository

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/traces/models"
)

// SpanEventTuple is one entry of the spans.events nested column.
type SpanEventTuple struct {
	Name         string            `ch:"name"`
	TimeUnixNano uint64            `ch:"time_unix_nano"`
	Attributes   map[string]string `ch:"attributes"`
}

// SpanLinkTuple is one entry of the spans.links nested column.
type SpanLinkTuple struct {
	TraceID    string            `ch:"trace_id"`
	SpanID     string            `ch:"span_id"`
	TraceState string            `ch:"trace_state"`
	Attributes map[string]string `ch:"attributes"`
}

// SpanEventCombinedRow carries a span's structured events alongside its
// exception columns, so both sources of "something happened" arrive in one
// read and the service can reconcile them.
type SpanEventCombinedRow struct {
	SpanID              string           `ch:"span_id"`
	TraceID             string           `ch:"trace_id"`
	Timestamp           time.Time        `ch:"timestamp"`
	Events              []SpanEventTuple `ch:"events"`
	ExceptionType       string           `ch:"exception_type"`
	ExceptionMessage    string           `ch:"exception_message"`
	ExceptionStacktrace string           `ch:"exception_stacktrace"`
}

// SpanAttributeRow is one span's full attribute payload.
type SpanAttributeRow struct {
	SpanID              string            `ch:"span_id"`
	TraceID             string            `ch:"trace_id"`
	OperationName       string            `ch:"operation_name"`
	ServiceName         string            `ch:"service"`
	Attributes          map[string]string `ch:"attributes"`
	ExceptionType       string            `ch:"exception_type"`
	ExceptionMessage    string            `ch:"exception_message"`
	ExceptionStacktrace string            `ch:"exception_stacktrace"`
	DBSystem            string            `ch:"db_system"`
	DBName              string            `ch:"db_name"`
	DBStatement         string            `ch:"db_statement"`
	Links               []SpanLinkTuple   `ch:"links"`
}

// TraceSummaryRow is the whole-trace aggregate. The service folds it into
// models.TraceSummary, which is where the derived duration is computed.
type TraceSummaryRow struct {
	TraceID        string    `ch:"trace_id"`
	StartTime      time.Time `ch:"start_time"`
	EndTime        time.Time `ch:"end_time"`
	RootService    string    `ch:"root_service"`
	RootOperation  string    `ch:"root_operation"`
	RootStatus     string    `ch:"root_status"`
	RootHTTPMethod string    `ch:"root_http_method"`
	RootHTTPStatus string    `ch:"root_http_status"`
	SpanCount      uint64    `ch:"span_count"`
	ErrorCount     uint64    `ch:"error_count"`
	HasError       bool      `ch:"trace_has_error"`
	ServiceSet     []string  `ch:"service_set"`
	RootMissing    bool      `ch:"root_missing"`
}

func (r *Repository) GetSpanEvents(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) ([]SpanEventCombinedRow, error) {
	const query = `
		SELECT span_id, trace_id, timestamp, events,
		       exception_type, exception_message, exception_stacktrace
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id = @traceID
		WHERE NOT empty(events) OR NOT empty(exception_type)`
	var rows []SpanEventCombinedRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "detail.GetSpanEvents", &rows, query,
		boundedTraceArgs(tenantID, traceID, startMs, endMs)...,
	)
	return rows, err
}

func (r *Repository) GetSpanAttributes(ctx context.Context, tenantID int64, traceID, spanID string, startMs, endMs int64) (*SpanAttributeRow, error) {
	const query = `
		SELECT span_id, trace_id, name AS operation_name, service,
		       attributes,
		       exception_type,
			   exception_message,
			   exception_stacktrace,
		       db_system,
			   db_name,
			   db_statement,
		       links AS links
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id = @traceID
		     AND span_id  = @spanID
		LIMIT 1`
	var row SpanAttributeRow
	args := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("traceID", traceID),
		clickhouse.Named("spanID", spanID),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
	}
	if err := dbutil.QueryRowCH(dbutil.ExplorerCtx(ctx), r.db, "detail.GetSpanAttributes", &row, query, args...); err != nil {
		return nil, err
	}
	if row.SpanID == "" {
		return nil, nil
	}
	return &row, nil
}

func (r *Repository) GetRelatedTraces(ctx context.Context, tenantID int64, serviceName, operationName string, startMs, endMs int64, excludeTraceID string, limit int) ([]models.RelatedTrace, error) {
	const query = `
		SELECT span_id,
		       trace_id,
		       name                       AS operation_name,
		       service,
		       duration_nano / 1000000.0  AS duration_ms,
		       status_code_string         AS status,
		       timestamp                  AS start_time
		FROM optikk.spans
		PREWHERE tenant_id      = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND is_root = 1
		     AND service      = @serviceName
		     AND name         = @operationName
		WHERE trace_id != @excludeTraceID
		ORDER BY timestamp DESC
		LIMIT @limit`
	args := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("operationName", operationName),
		clickhouse.Named("excludeTraceID", excludeTraceID),
		clickhouse.Named("limit", limit),
	}
	var rows []models.RelatedTrace
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "detail.GetRelatedTraces", &rows, query, args...)
	return rows, err
}

// GetTraceSummary aggregates the whole trace. A zero TraceID means no trace
// matched, which the service reports as not-found rather than an error.
func (r *Repository) GetTraceSummary(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) (*TraceSummaryRow, error) {
	const query = `
		SELECT trace_id,
		       min(timestamp)                                            AS start_time,
		       max(timestamp + toIntervalNanosecond(duration_nano))      AS end_time,
		       argMin(service,              timestamp)                   AS root_service,
		       argMin(name,                 timestamp)                   AS root_operation,
		       argMin(status_code_string,   timestamp)                   AS root_status,
		       argMin(http_method,          timestamp)                   AS root_http_method,
		       argMin(response_status_code, timestamp)                   AS root_http_status,
		       count()                                                   AS span_count,
		       countIf(is_error = 1)                                     AS error_count,
		       error_count > 0                                           AS trace_has_error,
		       groupUniqArray(service)                                   AS service_set,
		       countIf(is_root = 1) = 0                                  AS root_missing
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id = @traceID
		GROUP BY trace_id
		LIMIT 1`
	var res TraceSummaryRow
	if err := dbutil.QueryRowCH(dbutil.ExplorerCtx(ctx), r.db, "detail.GetTraceSummary", &res, query, boundedTraceArgs(tenantID, traceID, startMs, endMs)...); err != nil {
		return nil, err
	}
	if res.TraceID == "" {
		return nil, nil
	}
	return &res, nil
}

func (r *Repository) ListSpansByTrace(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) ([]models.SpanListItem, error) {
	const query = `
		SELECT span_id,
		       parent_span_id,
		       trace_id,
		       service,
		       name,
		       kind_string,
		       status_code_string,
		       is_error = 1                       AS has_error,
		       duration_nano / 1000000.0          AS duration_ms,
		       timestamp
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id = @traceID
		ORDER BY timestamp ASC
		LIMIT 5000`
	var rows []models.SpanListItem
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "detail.ListSpansByTrace", &rows, query,
		boundedTraceArgs(tenantID, traceID, startMs, endMs)...,
	)
	return rows, err
}

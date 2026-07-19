package detail

import (
	"context"
	"time"

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

func (r *Repository) GetSpanEvents(ctx context.Context, tenantID int64, traceID string, startTimeMs, endTimeMs int64) ([]spanEventCombinedRow, error) {
	const query = `
		SELECT span_id, trace_id, timestamp, events,
		       exception_type, exception_message, exception_stacktrace
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id = @traceID
		WHERE NOT empty(events) OR NOT empty(exception_type)`
	var rows []spanEventCombinedRow
	args := append(chargs.RangeArgs(tenantID, startTimeMs, endTimeMs), clickhouse.Named("traceID", traceID))
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "detail.GetSpanEvents", &rows, query,
		args...,
	)
	return rows, err
}

func (r *Repository) GetSpanAttributes(ctx context.Context, tenantID int64, traceID, spanID string, startTimeMs, endTimeMs int64) (*spanAttributeRow, error) {
	const query = `
		SELECT span_id, trace_id, name AS operation_name, service,
		       toJSONString(attributes)                AS attributes_json,
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
		     AND span_id  = @spanID
		     AND trace_id = @traceID
		LIMIT 1`
	var rows []spanAttributeRow
	args := append(chargs.RangeArgs(tenantID, startTimeMs, endTimeMs),
		clickhouse.Named("traceID", traceID),
		clickhouse.Named("spanID", spanID),
	)
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "detail.GetSpanAttributes", &rows, query, args...); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	return &row, nil
}

func (r *Repository) GetRelatedTraces(ctx context.Context, tenantID int64, serviceName, operationName string, startMs, endMs int64, excludeTraceID string, limit int) ([]RelatedTrace, error) {
	const query = `
		WITH active_fps AS (
		    SELECT fingerprint
		    FROM optikk.spans_resource
		    PREWHERE tenant_id = @tenantID
		         AND service  = @serviceName
		)
		SELECT span_id,
		       trace_id,
		       name                       AS operation_name,
		       service,
		       duration_nano / 1000000.0  AS duration_ms,
		       status_code_string         AS status,
		       timestamp                  AS start_time
		FROM optikk.spans
		PREWHERE tenant_id      = @tenantID
		     AND fingerprint  IN active_fps
		     AND service      = @serviceName
		     AND name         = @operationName
		WHERE is_root = 1
		  AND timestamp BETWEEN @start AND @end
		  AND trace_id != @excludeTraceID
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
	var rows []RelatedTrace
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "detail.GetRelatedTraces", &rows, query, args...)
	return rows, err
}

// GetTraceSummary aggregates the whole trace rather than reading its root span.
// A trace whose root was sampled out or dropped still has a summary; the
// earliest span stands in for the root and root_missing marks it as partial.
func (r *Repository) GetTraceSummary(ctx context.Context, tenantID int64, traceID string, startTimeMs, endTimeMs int64) (*TraceSummary, error) {
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
		       countIf(has_error)                                        AS error_count,
		       -- aliased away from the has_error column, which cannot be
		       -- shadowed by an aggregate over itself.
		       error_count > 0                                           AS trace_has_error,
		       groupUniqArray(service)                                   AS service_set,
		       countIf(is_root = 1) = 0                                  AS root_missing
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id = @traceID
		GROUP BY trace_id`
	var rows []struct {
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
	args := append(chargs.RangeArgs(tenantID, startTimeMs, endTimeMs), clickhouse.Named("traceID", traceID))
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "detail.GetTraceSummary", &rows, query,
		args...,
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	res := rows[0]
	return &TraceSummary{
		TraceID:        res.TraceID,
		StartMs:        uint64(res.StartTime.UnixMilli()),
		EndMs:          uint64(res.EndTime.UnixMilli()),
		DurationMs:     float64(res.EndTime.Sub(res.StartTime).Nanoseconds()) / 1_000_000,
		RootService:    res.RootService,
		RootOperation:  res.RootOperation,
		RootStatus:     res.RootStatus,
		RootHTTPMethod: res.RootHTTPMethod,
		RootHTTPStatus: res.RootHTTPStatus,
		SpanCount:      uint32(res.SpanCount),
		HasError:       res.HasError,
		ErrorCount:     uint32(res.ErrorCount),
		ServiceSet:     res.ServiceSet,
		RootMissing:    res.RootMissing,
	}, nil
}

func (r *Repository) ListSpansByTrace(ctx context.Context, tenantID int64, traceID string, startTimeMs, endTimeMs int64) ([]SpanListItem, error) {
	const query = `
		SELECT span_id,
		       parent_span_id,
		       trace_id,
		       service,
		       name,
		       kind_string,
		       status_code_string,
		       has_error,
		       duration_nano / 1000000.0          AS duration_ms,
		       timestamp
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id = @traceID
		ORDER BY timestamp ASC
		LIMIT 5000`
	var rows []SpanListItem
	args := append(chargs.RangeArgs(tenantID, startTimeMs, endTimeMs), clickhouse.Named("traceID", traceID))
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "detail.ListSpansByTrace", &rows, query,
		args...,
	)
	return rows, err
}

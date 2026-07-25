package errors

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ServiceErrorRateRowsAll(ctx context.Context, tenantID int64, startMs, endMs int64) ([]rawServiceRateRow, error) {
	query := `
		SELECT service                                           AS service_name,
		       ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.DurationSum + `
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		GROUP BY service_name, bucket_at
		ORDER BY bucket_at ASC
		LIMIT 10000`
	args := chargs.RollupRangeArgs(tenantID, startMs, endMs)
	var rows []rawServiceRateRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ServiceErrorRateAll", &rows, query, args...)
}

func (r *Repository) ServiceErrorRateRowsByService(ctx context.Context, tenantID int64, startMs, endMs int64, serviceName string) ([]rawServiceRateRow, error) {
	query := `
		SELECT service                                           AS service_name,
		       ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.DurationSum + `
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND service = @serviceName
		GROUP BY service_name, bucket_at
		ORDER BY bucket_at ASC
		LIMIT 10000`
	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
	)
	var rows []rawServiceRateRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ServiceErrorRateByService", &rows, query, args...)
}

func (r *Repository) ErrorVolumeRowsAll(ctx context.Context, tenantID int64, startMs, endMs int64) ([]rawServiceErrorRow, error) {
	query := `
		SELECT service                                           AS service_name,
		       ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       ` + spanstats.Errors + `
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		GROUP BY service_name, bucket_at
		ORDER BY bucket_at ASC
		LIMIT 10000`
	args := chargs.RollupRangeArgs(tenantID, startMs, endMs)
	var rows []rawServiceErrorRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorVolumeAll", &rows, query, args...)
}

func (r *Repository) ErrorVolumeRowsByService(ctx context.Context, tenantID int64, startMs, endMs int64, serviceName string) ([]rawServiceErrorRow, error) {
	query := `
		SELECT service                                           AS service_name,
		       ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       ` + spanstats.Errors + `
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND service = @serviceName
		GROUP BY service_name, bucket_at
		ORDER BY bucket_at ASC
		LIMIT 10000`
	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
	)
	var rows []rawServiceErrorRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorVolumeByService", &rows, query, args...)
}

func (r *Repository) ErrorGroupRowsAll(ctx context.Context, tenantID int64, startMs, endMs int64, limit int, cursor ErrorGroupsCursor) ([]rawErrorGroupRow, error) {
	var havingClause string
	if !cursor.IsZero() {
		havingClause = "HAVING (error_count < @cursorCount OR (error_count = @cursorCount AND error_group_id > @cursorID))"
	}

	query := `
		SELECT error_group_id                    AS error_group_id,
		       service                          AS service,
		       name                             AS operation_name,
		       http_status_bucket               AS http_status_bucket,
		       count()                          AS error_count,
		       max(timestamp)                   AS last_occurrence,
		       min(timestamp)                   AS first_occurrence,
		       anyLast(status_message)          AS status_message,
		       anyLast(trace_id)                AS sample_trace_id
		FROM optikk.spans
		PREWHERE tenant_id   = @tenantID AND timestamp BETWEEN @start AND @end AND is_error = 1
		GROUP BY error_group_id, service, name, http_status_bucket
		` + havingClause + `
		ORDER BY error_count DESC, error_group_id ASC
		LIMIT @limit`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("limit", limit),
		clickhouse.Named("cursorCount", cursor.ErrorCount),
		clickhouse.Named("cursorID", cursor.GroupID),
	)
	var rows []rawErrorGroupRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupsAll", &rows, query, args...)
}

func (r *Repository) ErrorGroupRowsByService(ctx context.Context, tenantID int64, startMs, endMs int64, serviceName string, limit int, cursor ErrorGroupsCursor) ([]rawErrorGroupRow, error) {
	var havingClause string
	if !cursor.IsZero() {
		havingClause = "HAVING (error_count < @cursorCount OR (error_count = @cursorCount AND error_group_id > @cursorID))"
	}

	query := `
		SELECT error_group_id                    AS error_group_id,
		       service                          AS service,
		       name                             AS operation_name,
		       http_status_bucket               AS http_status_bucket,
		       count()                          AS error_count,
		       max(timestamp)                   AS last_occurrence,
		       min(timestamp)                   AS first_occurrence,
		       anyLast(status_message)          AS status_message,
		       anyLast(trace_id)                AS sample_trace_id
		FROM optikk.spans
		PREWHERE tenant_id     = @tenantID AND timestamp BETWEEN @start AND @end AND is_error = 1 AND service = @serviceName
		GROUP BY error_group_id, service, name, http_status_bucket
		` + havingClause + `
		ORDER BY error_count DESC, error_group_id ASC
		LIMIT @limit`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("limit", limit),
		clickhouse.Named("cursorCount", cursor.ErrorCount),
		clickhouse.Named("cursorID", cursor.GroupID),
	)
	var rows []rawErrorGroupRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupsByService", &rows, query, args...)
}

func (r *Repository) ErrorGroupSamples(ctx context.Context, tenantID int64, startMs, endMs int64, groupIDs []string) ([]rawErrorGroupSampleRow, error) {
	const query = `
		SELECT error_group_id                    AS error_group_id,
		       anyLast(status_message)          AS status_message,
		       anyLast(trace_id)                AS sample_trace_id
		FROM optikk.spans
		PREWHERE tenant_id   = @tenantID AND timestamp BETWEEN @start AND @end AND is_error = 1 AND error_group_id IN @groupIDs
		GROUP BY error_group_id`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("groupIDs", groupIDs),
	)
	var rows []rawErrorGroupSampleRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupSamples", &rows, query, args...)
}

func (r *Repository) ErrorGroupDetailRow(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) (*rawErrorGroupDetailRow, error) {
	const query = `
		SELECT error_group_id                       AS error_group_id,
		       service                              AS service,
		       name                                 AS operation_name,
		       toUInt16OrZero(any(response_status_code)) AS http_status_code,
		       count()                                   AS error_count,
		       max(timestamp)                       AS last_occurrence,
		       min(timestamp)                       AS first_occurrence,
		       any(exception_type)                  AS exception_type
		FROM optikk.spans
		PREWHERE tenant_id     = @tenantID AND timestamp BETWEEN @start AND @end AND is_error = 1 AND error_group_id = @groupID
		GROUP BY error_group_id, service, name`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("groupID", groupID),
	)
	var row rawErrorGroupDetailRow
	if err := dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupDetail", &row, query, args...); err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repository) ErrorGroupTraceRows(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string, limit int, cursor ErrorTracesCursor) ([]rawErrorGroupTraceRow, error) {
	var paginationFilter string
	if !cursor.IsZero() {
		paginationFilter = "AND (s.timestamp < @cursorTs OR (s.timestamp = @cursorTs AND s.span_id > @cursorSpan))"
	}

	query := `
		SELECT s.trace_id                       AS trace_id,
		       s.span_id                        AS span_id,
		       s.timestamp                      AS timestamp,
		       s.duration_nano / 1000000.0      AS duration_ms,
		       s.status_code_string             AS status_code
		FROM optikk.spans s
		PREWHERE s.tenant_id     = @tenantID AND s.timestamp BETWEEN @start AND @end AND s.is_error = 1 AND s.error_group_id = @groupID
		WHERE 1=1 ` + paginationFilter + `
		ORDER BY s.timestamp DESC, s.span_id ASC
		LIMIT @limit`
	args := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("groupID", groupID),
		clickhouse.Named("limit", limit),
		clickhouse.Named("cursorTs", cursor.Timestamp),
		clickhouse.Named("cursorSpan", cursor.SpanID),
	}
	var rows []rawErrorGroupTraceRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupTraces", &rows, query, args...)
}

func (r *Repository) ErrorGroupTimeseriesRows(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) ([]rawTimeBucketCountRow, error) {
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       count()                            AS count
		FROM optikk.spans
		PREWHERE tenant_id     = @tenantID AND timestamp BETWEEN @start AND @end AND is_error = 1 AND error_group_id = @groupID
		GROUP BY bucket_at
		HAVING count > 0
		ORDER BY bucket_at ASC`
	args := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("groupID", groupID),
	}
	var rows []rawTimeBucketCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupTimeseries", &rows, query, args...)
}

func (r *Repository) ErrorGroupLatestOccurrenceRow(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) (*rawErrorLatestOccurrenceRow, error) {
	const query = `
		SELECT s.trace_id                  AS trace_id,
		       s.span_id                   AS span_id,
		       s.timestamp                 AS timestamp,
		       s.duration_nano / 1000000.0 AS duration_ms,
		       s.exception_message         AS exception_message,
		       s.exception_stacktrace      AS exception_stacktrace,
		       s.http_method               AS http_method,
		       s.http_route                AS http_route,
		       s.response_status_code      AS response_status_code,
		       s.service_version           AS service_version,
		       s.environment               AS environment,
		       s.pod                       AS pod,
		       s.host                      AS host
		FROM optikk.spans s
		PREWHERE s.tenant_id     = @tenantID AND s.timestamp BETWEEN @start AND @end AND s.is_error = 1 AND s.error_group_id = @groupID
		ORDER BY s.timestamp DESC
		LIMIT 1`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("groupID", groupID),
	)
	var row rawErrorLatestOccurrenceRow
	if err := dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupLatestOccurrence", &row, query, args...); err != nil {
		return nil, err
	}
	return &row, nil
}

type rawErrorFacetGroupRow struct {
	Dim   string `ch:"dim"`
	Value string `ch:"value"`
	Count uint64 `ch:"cnt"`
}

func (r *Repository) ErrorGroupFacetRowsAll(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) ([]rawErrorFacetGroupRow, error) {
	const query = `
		SELECT
			multiIf(
				grouping(service_version) = 0, 'service_version',
				grouping(environment) = 0, 'environment',
				grouping(pod) = 0, 'pod',
				grouping(http_route) = 0, 'http_route',
				''
			) as dim,
			multiIf(
				grouping(service_version) = 0, service_version,
				grouping(environment) = 0, environment,
				grouping(pod) = 0, pod,
				grouping(http_route) = 0, http_route,
				''
			) as value,
			count() as cnt
		FROM optikk.spans
		PREWHERE tenant_id     = @tenantID AND timestamp BETWEEN @start AND @end AND is_error = 1 AND error_group_id = @groupID
		GROUP BY GROUPING SETS (
			(service_version),
			(environment),
			(pod),
			(http_route)
		)
		HAVING value != ''
		ORDER BY dim, cnt DESC
		LIMIT 8 BY dim`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("groupID", groupID),
	)
	var rows []rawErrorFacetGroupRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupFacetAll", &rows, query, args...)
}

func (r *Repository) ErrorHotspotRows(ctx context.Context, tenantID int64, startMs, endMs int64) ([]rawErrorHotspotRow, error) {
	query := `
		SELECT service,
		       any(name)                 AS operation_name,
		       error_group_id,
		       count()                   AS error_count
		FROM optikk.spans
		PREWHERE tenant_id   = @tenantID AND timestamp BETWEEN @start AND @end AND is_error = 1
		WHERE name != ''
		GROUP BY service, error_group_id
		ORDER BY error_count DESC
		LIMIT 2 BY service`
	args := chargs.RangeArgs(tenantID, startMs, endMs)
	var rows []rawErrorHotspotRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorHotspot", &rows, query, args...)
}

package repository

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/services/errors/models"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/errorgroups"
)

func (r *Repository) ErrorGroupDetailRow(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) (*models.RawErrorGroupDetailRow, error) {
	query := `
		SELECT ` + errorgroups.IdentityProjection("") + `,
		       service                              AS service,
		       toUInt16OrZero(argMax(response_status_code, (timestamp, span_id))) AS http_status_code,
		       count()                                   AS error_count,
		       max(timestamp)                       AS last_occurrence,
		       min(timestamp)                       AS first_occurrence
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		     AND ` + errorgroups.Predicate + ` AND error_group_id = @groupID
		GROUP BY error_group_id, service, name`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("groupID", groupID),
	)
	var row models.RawErrorGroupDetailRow
	if err := dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupDetail", &row, query, args...); err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repository) ErrorGroupTraceRows(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string, limit int, cursor models.ErrorTracesCursor) ([]models.RawErrorGroupTraceRow, error) {
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
		PREWHERE s.tenant_id = @tenantID AND s.timestamp >= @start AND s.timestamp < @end
		     AND ` + errorgroups.QualifiedPredicate("s") + ` AND s.error_group_id = @groupID
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
	var rows []models.RawErrorGroupTraceRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupTraces", &rows, query, args...)
}

func (r *Repository) ErrorGroupTimeseriesRows(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) ([]models.RawTimeBucketCountRow, error) {
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       count()                            AS count
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		     AND ` + errorgroups.Predicate + ` AND error_group_id = @groupID
		GROUP BY bucket_at
		HAVING count > 0
		ORDER BY bucket_at ASC`
	args := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("groupID", groupID),
	}
	var rows []models.RawTimeBucketCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupTimeseries", &rows, query, args...)
}

func (r *Repository) ErrorGroupLatestOccurrenceRow(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) (*models.RawErrorLatestOccurrenceRow, error) {
	query := `
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
		PREWHERE s.tenant_id = @tenantID AND s.timestamp >= @start AND s.timestamp < @end
		     AND ` + errorgroups.QualifiedPredicate("s") + ` AND s.error_group_id = @groupID
		ORDER BY s.timestamp DESC, s.span_id DESC
		LIMIT 1`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("groupID", groupID),
	)
	var row models.RawErrorLatestOccurrenceRow
	if err := dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupLatestOccurrence", &row, query, args...); err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repository) ErrorGroupFacetRowsAll(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) ([]models.RawFacetDimRow, error) {
	query := `
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
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		     AND ` + errorgroups.Predicate + ` AND error_group_id = @groupID
		GROUP BY GROUPING SETS (
			(service_version),
			(environment),
			(pod),
			(http_route)
		)
		HAVING value != ''
		ORDER BY dim, cnt DESC, value ASC
		LIMIT 8 BY dim`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("groupID", groupID),
	)
	var rows []models.RawFacetDimRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupFacetAll", &rows, query, args...)
}

func (r *Repository) ErrorHotspotRows(ctx context.Context, tenantID int64, startMs, endMs int64) ([]models.RawErrorHotspotRow, error) {
	query := `
		SELECT service,
		       argMax(name, (timestamp, span_id)) AS operation_name,
		       error_group_id,
		       count()                   AS error_count
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		     AND ` + errorgroups.Predicate + `
		WHERE name != ''
		GROUP BY service, error_group_id
		ORDER BY error_count DESC, service ASC, error_group_id ASC
		LIMIT 2 BY service`
	args := chargs.RangeArgs(tenantID, startMs, endMs)
	var rows []models.RawErrorHotspotRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorHotspot", &rows, query, args...)
}

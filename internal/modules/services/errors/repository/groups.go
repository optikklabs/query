package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/services/errors/models"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/errorgroups"
)

// ErrorGroupDetailRow returns sql.ErrNoRows when the group has no errors in
// the window.
func (r *Repository) ErrorGroupDetailRow(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) (models.RawErrorGroupDetailRow, error) {
	query := `
		SELECT ` + errorgroups.IdentityProjection("") + `,
		       service                              AS service,
		       toUInt16OrNull(argMax(response_status_code, (timestamp, span_id))) AS http_status_code,
		       count()                                   AS error_count,
		       max(timestamp)                       AS last_occurrence,
		       min(timestamp)                       AS first_occurrence
		FROM optikk.error_events
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		     AND ` + errorgroups.Predicate + ` AND error_group_id = @groupID
		GROUP BY error_group_id, service, name`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("groupID", groupID),
	)
	var row models.RawErrorGroupDetailRow
	err := dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupDetail", &row, query, args...)
	return row, err
}

func (r *Repository) ErrorGroupTraceRows(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string, limit int, cursor *models.ErrorTracesCursor) ([]models.RawErrorGroupTraceRow, error) {
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("groupID", groupID),
		clickhouse.Named("limit", limit),
	)
	var paginationFilter string
	if cursor != nil {
		paginationFilter = "AND (s.timestamp < @cursorTs OR (s.timestamp = @cursorTs AND s.span_id > @cursorSpan))"
		args = append(args,
			chargs.Nanos("cursorTs", cursor.Timestamp),
			clickhouse.Named("cursorSpan", cursor.SpanID),
		)
	}

	query := `
		SELECT s.trace_id                       AS trace_id,
		       s.span_id                        AS span_id,
		       s.timestamp                      AS timestamp,
		       s.duration_nano / 1000000.0      AS duration_ms,
		       s.status_code_string             AS status_code
		FROM optikk.error_events s
		PREWHERE s.tenant_id = @tenantID AND s.timestamp >= @start AND s.timestamp < @end
		     AND ` + errorgroups.QualifiedPredicate("s") + ` AND s.error_group_id = @groupID
		WHERE 1=1 ` + paginationFilter + `
		ORDER BY s.timestamp DESC, s.span_id ASC
		LIMIT @limit`
	var rows []models.RawErrorGroupTraceRow
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupTraces", &rows, query, args...)
	return rows, err
}

func (r *Repository) ErrorGroupTimeseriesRows(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) ([]models.RawTimeBucketCountRow, error) {
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       count()                            AS count
		FROM optikk.error_events
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		     AND ` + errorgroups.Predicate + ` AND error_group_id = @groupID
		GROUP BY bucket_at
		HAVING count > 0
		ORDER BY bucket_at ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("groupID", groupID),
	)
	var rows []models.RawTimeBucketCountRow
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupTimeseries", &rows, query, args...)
	return rows, err
}

// ErrorGroupLatestOccurrenceRow returns sql.ErrNoRows when the group has no
// errors in the window.
func (r *Repository) ErrorGroupLatestOccurrenceRow(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) (models.RawErrorLatestOccurrenceRow, error) {
	query := `
		SELECT s.trace_id                  AS trace_id,
		       s.span_id                   AS span_id,
		       s.timestamp                 AS timestamp,
		       s.duration_nano / 1000000.0 AS duration_ms,
		       s.exception_message         AS exception_message,
		       s.exception_stacktrace      AS exception_stacktrace,
		       s.http_method               AS http_method,
		       s.http_route                AS http_route,
		       toUInt16OrNull(s.response_status_code) AS http_status_code,
		       s.service_version           AS service_version,
		       s.environment               AS environment,
		       s.pod                       AS pod,
		       s.host                      AS host
		FROM optikk.error_events s
		PREWHERE s.tenant_id = @tenantID AND s.timestamp >= @start AND s.timestamp < @end
		     AND ` + errorgroups.QualifiedPredicate("s") + ` AND s.error_group_id = @groupID
		ORDER BY s.timestamp DESC, s.span_id DESC
		LIMIT 1`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("groupID", groupID),
	)
	var row models.RawErrorLatestOccurrenceRow
	err := dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupLatestOccurrence", &row, query, args...)
	return row, err
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
		FROM optikk.error_events
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
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorGroupFacetAll", &rows, query, args...)
	return rows, err
}

func (r *Repository) ErrorHotspotRows(ctx context.Context, tenantID int64, startMs, endMs int64) ([]models.RawErrorHotspotRow, error) {
	query := `
		SELECT service,
		       argMax(name, (timestamp, span_id)) AS operation_name,
		       error_group_id,
		       count()                   AS error_count
		FROM optikk.error_events
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		     AND ` + errorgroups.Predicate + `
		WHERE name != ''
		GROUP BY service, error_group_id
		ORDER BY error_count DESC, service ASC, error_group_id ASC
		LIMIT 2 BY service`
	args := chargs.RangeArgs(tenantID, startMs, endMs)
	var rows []models.RawErrorHotspotRow
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ErrorHotspot", &rows, query, args...)
	return rows, err
}

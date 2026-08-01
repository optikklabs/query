package repository

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/infra/cursor"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/services/errors/models"
	"github.com/optikklabs/query/internal/shared/errorgroups"
	"github.com/optikklabs/query/internal/shared/spanfilter"
)

func decodeGroupsCursor(raw string) (models.ErrorGroupsCursor, bool) {
	return cursor.Decode[models.ErrorGroupsCursor](raw)
}

func millisToTime(ms int64) time.Time { return time.UnixMilli(ms) }

// Error groups are always read from the error spans themselves, so span- and
// root-level predicates both apply to the same row — no trace-level CTE.
const errorSpanScan = `FROM optikk.spans
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end AND ` + errorgroups.Predicate + `
		WHERE 1=1`

func errorSpanWhere(f spanfilter.Filters) (string, []any) {
	c := spanfilter.BuildClauses(f)
	return errorSpanScan + c.Span + c.Root, c.Args
}

func (r *Repository) ExplorerGroupRows(ctx context.Context, req models.GroupsRequest) ([]models.RawErrorGroupRow, error) {
	scan, args := errorSpanWhere(req.Filters)

	var having string
	if cur, ok := decodeGroupsCursor(req.Cursor); ok {
		having = `HAVING (error_count < @cursorCount OR (error_count = @cursorCount AND error_group_id > @cursorID))`
		args = append(args,
			clickhouse.Named("cursorCount", cur.ErrorCount),
			clickhouse.Named("cursorID", cur.GroupID),
		)
	}
	args = append(args, clickhouse.Named("pgLimit", uint64(req.Limit)))

	query := `
		SELECT error_group_id           AS error_group_id,
		       service                  AS service,
		       name                     AS operation_name,
		       http_status_bucket       AS http_status_bucket,
		       count()                  AS error_count,
		       max(timestamp)           AS last_occurrence,
		       min(timestamp)           AS first_occurrence,
		       -- Aliased away from the column name: a status_message alias
		       -- shadows the column for the message filter in WHERE.
		       argMax(status_message, (timestamp, span_id)) AS error_message,
		       argMax(trace_id, (timestamp, span_id))       AS sample_trace_id
		` + scan + `
		GROUP BY error_group_id, service, name, http_status_bucket
		` + having + `
		ORDER BY error_count DESC, error_group_id ASC
		LIMIT @pgLimit`

	var rows []models.RawErrorGroupRow
	return rows, dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "errors.ExplorerGroups", &rows, query, args...)
}

func (r *Repository) ExplorerFacetRows(ctx context.Context, req models.FacetsRequest) ([]models.RawFacetDimRow, error) {
	scan, args := errorSpanWhere(req.Filters)

	query := `
		SELECT multiIf(
		           grouping(service) = 0,              'service',
		           grouping(name) = 0,                 'operation',
		           grouping(response_status_code) = 0, 'httpStatus',
		           grouping(exception_type) = 0,       'exceptionType',
		           ''
		       ) AS dim,
		       multiIf(
		           grouping(service) = 0,              service,
		           grouping(name) = 0,                 name,
		           grouping(response_status_code) = 0, response_status_code,
		           grouping(exception_type) = 0,       exception_type,
		           ''
		       ) AS value,
		       count() AS cnt
		` + scan + `
		GROUP BY GROUPING SETS ((service), (name), (response_status_code), (exception_type))
		HAVING value != ''
		ORDER BY dim, cnt DESC, value ASC
		LIMIT 20 BY dim`

	var rows []models.RawFacetDimRow
	return rows, dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "errors.ExplorerFacets", &rows, query, args...)
}

// ExplorerSummaryRow aggregates over groups, not spans, so "active"/"new"
// issue counts are exact for the range instead of a sample of the first page.
func (r *Repository) ExplorerSummaryRow(ctx context.Context, req models.OverviewRequest, newSinceMs int64) (models.RawSummaryRow, error) {
	scan, args := errorSpanWhere(req.Filters)
	args = append(args, clickhouse.DateNamed("newSince", millisToTime(newSinceMs), clickhouse.MilliSeconds))

	query := `
		SELECT sum(cnt)                        AS total_errors,
		       count()                         AS active_issues,
		       countIf(first_occ >= @newSince) AS new_issues,
		       uniqExact(service)              AS services_affected
		FROM (
		    SELECT min(service)   AS service,
		           count()        AS cnt,
		           min(timestamp) AS first_occ
		    ` + scan + `
		    GROUP BY error_group_id
		)`

	var row models.RawSummaryRow
	err := dbutil.QueryRowCH(dbutil.ExplorerCtx(ctx), r.db, "errors.ExplorerSummary", &row, query, args...)
	return row, err
}

func (r *Repository) ExplorerTrendRows(ctx context.Context, req models.OverviewRequest) ([]models.RawTrendRow, error) {
	scan, args := errorSpanWhere(req.Filters)

	query := `
		SELECT ` + timebucket.DisplayGrainSQL(req.EndTime-req.StartTime) + ` AS time_bucket,
		       count()                                                      AS errors
		` + scan + `
		GROUP BY time_bucket
		ORDER BY time_bucket ASC`

	var rows []models.RawTrendRow
	return rows, dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "errors.ExplorerTrend", &rows, query, args...)
}

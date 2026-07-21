package explorer

import (
	"context"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/traces/filter"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// traceIndexColumns selects only what the root span actually knows. Trace-level
// facts (span count, error count, services, real end time) come from
// EnrichTraces; fabricating them here made every row report 1 span.
const traceIndexColumns = `trace_id,
		span_id,
		timestamp                                                  AS start_time,
		duration_nano                                              AS duration_ns,
		service                                                    AS root_service,
		name                                                       AS root_operation,
		status_code_string                                         AS root_status,
		http_method                                                AS root_http_method,
		response_status_code                                       AS root_http_status`

// rootScanParts assembles the shared core of every explorer query: an
// optional WITH prologue plus the PREWHERE/WHERE over root spans. Span-level
// filters run in an any-span trace_id subquery so a match on a child span
// still surfaces its trace; trace-level filters stay on the root scan.
func rootScanParts(c filter.Clauses) (with, prewhere, where string) {
	var ctes []string
	prewhere = `
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND is_root = 1`
	where = `
		WHERE 1=1` + c.Root
	if c.HasSpanMatch() {
		inner := `SELECT DISTINCT trace_id
		    FROM optikk.spans
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end`
		if c.Resource != "" {
			ctes = append(ctes, `match_fps AS (
		    SELECT DISTINCT fingerprint
		    FROM optikk.spans_resource
		    PREWHERE tenant_id = @tenantID`+c.Resource+`
		)`)
			inner += ` AND fingerprint IN match_fps`
		}
		ctes = append(ctes, `matched AS (
		    `+inner+`
		    WHERE 1=1`+c.Span+`
		)`)
		where += ` AND trace_id IN matched`
	}
	if c.ExcludeResource != "" {
		ctes = append(ctes, `keep_fps AS (
		    SELECT DISTINCT fingerprint
		    FROM optikk.spans_resource
		    PREWHERE tenant_id = @tenantID`+c.ExcludeResource+`
		)`)
		prewhere += ` AND fingerprint IN keep_fps`
	}
	if len(ctes) > 0 {
		with = `
		WITH ` + strings.Join(ctes, `, `)
	}
	return with, prewhere, where
}

func (r *Repository) Query(ctx context.Context, req QueryRequest) ([]traceIndexRowDTO, bool, error) {
	clauses := filter.BuildClauses(req.Filters)
	args := clauses.Args
	with, prewhere, where := rootScanParts(clauses)
	cur, _ := DecodeCursor(req.Cursor)
	if !cur.IsZero() {
		where += ` AND (timestamp, span_id) < (@curStart, @curSpanID)`
		args = append(args,
			// DateNamed with ns scale; a plain time.Time arg truncates to seconds.
			clickhouse.DateNamed("curStart", time.Unix(0, int64(cur.StartNs)), clickhouse.NanoSeconds),
			clickhouse.Named("curSpanID", cur.SpanID),
		)
	}
	args = append(args, clickhouse.Named("pgLimit", uint64(req.Limit+1)))

	query := with + `
		SELECT ` + traceIndexColumns + `
		FROM optikk.spans` + prewhere + where + `
		ORDER BY timestamp DESC, span_id DESC
		LIMIT @pgLimit`

	var rows []traceIndexRowDTO
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "traces.Query", &rows, query, args...); err != nil {
		return nil, false, err
	}

	hasMore := len(rows) > req.Limit
	if hasMore {
		rows = rows[:req.Limit]
	}
	return rows, hasMore, nil
}

// EnrichTraces returns the trace-level aggregates for one already-paginated
// page of trace ids. Bounded to the page (not the window), and cheap because
// idx_trace_id skips granules — one extra round trip, never an N+1.
func (r *Repository) EnrichTraces(ctx context.Context, tenantID int64, traceIDs []string) (map[string]traceAggRow, error) {
	if len(traceIDs) == 0 {
		return nil, nil
	}
	const query = `
		SELECT trace_id,
		       count()                                              AS span_count,
		       countIf(is_error = 1)                                AS error_count,
		       min(timestamp)                                       AS start_time,
		       max(timestamp + toIntervalNanosecond(duration_nano)) AS end_time,
		       groupUniqArray(service)                              AS service_set
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND trace_id IN @traceIDs
		GROUP BY trace_id`

	var rows []traceAggRow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "traces.EnrichTraces", &rows, query,
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("traceIDs", traceIDs),
	); err != nil {
		return nil, err
	}
	byTrace := make(map[string]traceAggRow, len(rows))
	for _, row := range rows {
		byTrace[row.TraceID] = row
	}
	return byTrace, nil
}

func (r *Repository) QueryFacets(ctx context.Context, req FacetsRequest) (Facets, error) {
	clauses := filter.BuildClauses(req.Filters)
	args := clauses.Args
	with, prewhere, where := rootScanParts(clauses)

	query := with + `
		SELECT
			multiIf(
				grouping(service) = 0, 'service',
				grouping(name) = 0, 'operation',
				grouping(http_method) = 0, 'http_method',
				grouping(response_status_code) = 0, 'http_status',
				grouping(status_code_string) = 0, 'status',
				''
			) as dim,
			multiIf(
				grouping(service) = 0, service,
				grouping(name) = 0, name,
				grouping(http_method) = 0, http_method,
				grouping(response_status_code) = 0, response_status_code,
				grouping(status_code_string) = 0, status_code_string,
				''
			) as value,
			count() as cnt
		FROM optikk.spans` + prewhere + where + `
		GROUP BY GROUPING SETS (
			(service),
			(name),
			(http_method),
			(response_status_code),
			(status_code_string)
		)
		HAVING value != ''
		ORDER BY dim, cnt DESC
		LIMIT 20 BY dim`

	var rows []facetDimRow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "facets.QueryFacets", &rows, query, args...); err != nil {
		return Facets{}, err
	}
	return pivotFacets(rows), nil
}

func pivotFacets(rows []facetDimRow) Facets {
	var f Facets
	for _, row := range rows {
		b := FacetBucket{Value: row.Value, Count: row.Count}
		switch row.Dim {
		case "service":
			f.Service = append(f.Service, b)
		case "operation":
			f.Operation = append(f.Operation, b)
		case "http_method":
			f.HTTPMethod = append(f.HTTPMethod, b)
		case "http_status":
			f.HTTPStatus = append(f.HTTPStatus, b)
		case "status":
			f.Status = append(f.Status, b)
		}
	}
	return f
}

func (r *Repository) QueryTrend(ctx context.Context, req TrendRequest) ([]TrendBucket, error) {
	clauses := filter.BuildClauses(req.Filters)
	args := clauses.Args
	with, prewhere, where := rootScanParts(clauses)
	grainSQL := timebucket.DisplayGrainSQL(req.EndTime - req.StartTime)

	query := with + `
		SELECT ` + grainSQL + `                          AS time_bucket,
		       count()                                   AS total,
		       countIf(is_error = 1)                     AS errors
		FROM optikk.spans` + prewhere + where + `
		GROUP BY time_bucket
		ORDER BY time_bucket ASC`

	var rows []struct {
		TimeBucket time.Time `ch:"time_bucket"`
		Total      uint64    `ch:"total"`
		Errors     uint64    `ch:"errors"`
	}
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "trend.QueryTrend", &rows, query, args...); err != nil {
		return nil, err
	}
	out := make([]TrendBucket, len(rows))
	for i, r := range rows {
		out[i] = TrendBucket{
			TimeBucket: timebucket.FormatDisplayBucket(r.TimeBucket),
			Total:      r.Total,
			Errors:     r.Errors,
		}
	}
	return out, nil
}

func (r *Repository) SuggestScalar(ctx context.Context, tenantID, startMs, endMs int64, field, prefix string, limit int) ([]Suggestion, error) {
	column := scalarFieldExpr(field)
	query := `
		SELECT ` + column + `        AS value,
		       count()               AS count
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @startMs AND @endMs
		WHERE ` + column + ` != ''
		  AND (length(@prefix) = 0 OR positionCaseInsensitive(value, @prefix) > 0)
		GROUP BY value
		ORDER BY count DESC
		LIMIT @limit`
	var rows []suggestionRow
	if err := dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "suggest.SuggestScalar", &rows, query, suggestArgs(tenantID, startMs, endMs, prefix, limit)...); err != nil {
		return nil, err
	}
	return filterutil.MapSuggestionRows(rows), nil
}

func (r *Repository) SuggestAttribute(ctx context.Context, tenantID, startMs, endMs int64, attrKey, prefix string, limit int) ([]Suggestion, error) {
	const query = `
		SELECT attributes[@attrKey] AS value, count() AS count
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @startMs AND @endMs
		WHERE value != ''
		  AND (length(@prefix) = 0 OR positionCaseInsensitive(value, @prefix) > 0)
		GROUP BY value
		ORDER BY count DESC
		LIMIT @limit`
	args := append(suggestArgs(tenantID, startMs, endMs, prefix, limit),
		clickhouse.Named("attrKey", strings.TrimPrefix(attrKey, "@")),
	)
	var rows []suggestionRow
	if err := dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "suggest.SuggestAttribute", &rows, query, args...); err != nil {
		return nil, err
	}
	return filterutil.MapSuggestionRows(rows), nil
}

func suggestArgs(tenantID, startMs, endMs int64, prefix string, limit int) []any {
	return []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("startMs", time.UnixMilli(startMs)),
		clickhouse.Named("endMs", time.UnixMilli(endMs)),
		clickhouse.Named("prefix", prefix),
		clickhouse.Named("limit", uint64(limit)),
	}
}

func scalarFieldExpr(field string) string {
	switch field {
	case "service":
		return "service"
	case "operation":
		return "name"
	case "http_method":
		return "http_method"
	case "http_status":
		return "response_status_code"
	case "status":
		return "status_code_string"
	case "environment":
		return "environment"
	default:
		return "''"
	}
}

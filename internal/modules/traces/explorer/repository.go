package explorer

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/filterutil"
	"github.com/optikklabs/query/internal/shared/spanfilter"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func buildScanClauses(c spanfilter.Clauses) (queryPrefix, prewhere, where string) {
	var ctes []string
	prewhere = `PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end`
	where = `WHERE 1=1` + c.Root

	if c.HasSpanMatch() {
		inner := `SELECT trace_id FROM optikk.spans PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end` + c.Resource
		ctes = append(ctes, `matched AS (`+inner+` WHERE 1=1`+c.Span+
			` GROUP BY trace_id ORDER BY max(timestamp) DESC LIMIT `+
			strconv.Itoa(filterutil.MaxMatchedTraces)+`)`)
		where += ` AND trace_id IN matched`
	}

	if len(ctes) > 0 {
		queryPrefix = `WITH ` + strings.Join(ctes, `, `) + `
`
	}
	return queryPrefix, prewhere, where
}

func (r *Repository) Query(ctx context.Context, req QueryRequest) ([]traceIndexRowDTO, error) {
	c := spanfilter.BuildClauses(req.Filters)
	prefix, prewhere, where := buildScanClauses(c)
	args := c.Args

	if cur, _ := DecodeCursor(req.Cursor); !cur.IsZero() {
		where += ` AND (timestamp, span_id) < (@curStart, @curSpanID)`
		args = append(args,
			clickhouse.DateNamed("curStart", time.Unix(0, int64(cur.StartNs)), clickhouse.NanoSeconds),
			clickhouse.Named("curSpanID", cur.SpanID),
		)
	}
	args = append(args, clickhouse.Named("pgLimit", uint64(req.Limit+1)))

	query := prefix + `SELECT
			trace_id,
			span_id,
			timestamp                                                  AS start_time,
			duration_nano                                              AS duration_ns,
			service                                                    AS root_service,
			name                                                       AS root_operation,
			status_code_string                                         AS root_status,
			http_method                                                AS root_http_method,
			response_status_code                                       AS root_http_status
		FROM optikk.spans_root ` + prewhere + ` ` + where + ` ORDER BY timestamp DESC, span_id DESC LIMIT @pgLimit`

	var rows []traceIndexRowDTO
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "traces.Query", &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) EnrichTraces(ctx context.Context, tenantID int64, traceIDs []string, start, end time.Time) ([]traceAggRow, error) {
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
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id IN @traceIDs
		GROUP BY trace_id
		LIMIT 500`

	var rows []traceAggRow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "traces.EnrichTraces", &rows, query,
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("start", start),
		clickhouse.Named("end", end),
		clickhouse.Named("traceIDs", traceIDs),
	); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) QueryFacets(ctx context.Context, req FacetsRequest) ([]facetDimRow, error) {
	c := spanfilter.BuildClauses(req.Filters)
	prefix, prewhere, where := buildScanClauses(c)
	query := prefix + `SELECT
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
		FROM optikk.spans_root ` + prewhere + ` ` + where + `
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
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "facets.QueryFacets", &rows, query, c.Args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) QueryTrend(ctx context.Context, req TrendRequest) ([]trendRow, error) {
	c := spanfilter.BuildClauses(req.Filters)
	prefix, prewhere, where := buildScanClauses(c)
	grainSQL := timebucket.DisplayGrainSQL(req.EndTime - req.StartTime)
	query := prefix + `SELECT ` + grainSQL + ` AS time_bucket, count() AS total, countIf(is_error = 1) AS errors FROM optikk.spans_root ` + prewhere + ` ` + where + ` GROUP BY time_bucket ORDER BY time_bucket ASC`

	var rows []trendRow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "trend.QueryTrend", &rows, query, c.Args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) SuggestScalar(ctx context.Context, tenantID, startMs, endMs int64, field, prefix string, limit int) ([]suggestionRow, error) {
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
	return rows, nil
}

func (r *Repository) SuggestAttribute(ctx context.Context, tenantID, startMs, endMs int64, attrKey, prefix string, limit int) ([]suggestionRow, error) {
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
	return rows, nil
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

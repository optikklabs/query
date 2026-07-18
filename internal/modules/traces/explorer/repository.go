package explorer

import (
	"context"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/traces/filter"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

const traceIndexColumns = `trace_id,
		span_id,
		timestamp                                                  AS start_time,
		timestamp                                                  AS end_time,
		duration_nano                                              AS duration_ns,
		service                                                    AS root_service,
		name                                                       AS root_operation,
		status_code_string                                         AS root_status,
		http_method                                                AS root_http_method,
		response_status_code                                       AS root_http_status,
		1                                                          AS span_count,
		has_error,
		(CASE WHEN has_error THEN 1 ELSE 0 END)                    AS error_count,
		[service]                                                  AS service_set,
		false                                                      AS truncated,
		timestamp                                                  AS last_seen`

func (r *Repository) Query(ctx context.Context, req QueryRequest) ([]traceIndexRowDTO, bool, error) {
	resourceWhere, where, args := filter.BuildClauses(req.Filters)
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

	var query string
	if resourceWhere == "" {
		query = `
			SELECT ` + traceIndexColumns + `
			FROM optikk.spans
			PREWHERE tenant_id = @tenantID
			WHERE timestamp BETWEEN @start AND @end AND is_root = 1` + where + `
			ORDER BY timestamp DESC, span_id DESC
			LIMIT @pgLimit`
	} else {
		query = `
			WITH active_fps AS (
			    SELECT DISTINCT fingerprint
			    FROM optikk.spans_resource
			    PREWHERE tenant_id = @tenantID` + resourceWhere + `
			)
			SELECT ` + traceIndexColumns + `
			FROM optikk.spans
			PREWHERE tenant_id = @tenantID AND fingerprint IN active_fps
			WHERE timestamp BETWEEN @start AND @end AND is_root = 1` + where + `
			ORDER BY timestamp DESC, span_id DESC
			LIMIT @pgLimit`
	}

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

func (r *Repository) QueryFacets(ctx context.Context, req FacetsRequest) (Facets, error) {
	resourceWhere, where, args := filter.BuildClauses(req.Filters)

	cte, prewhereFP := "", ""
	if resourceWhere != "" {
		cte = `
			WITH active_fps AS (
			    SELECT DISTINCT fingerprint
			    FROM optikk.spans_resource
			    PREWHERE tenant_id = @tenantID` + resourceWhere + `
			)`
		prewhereFP = " AND fingerprint IN active_fps"
	}

	query := cte + `
		SELECT 
			multiIf(service != '', 'service',
					name != '', 'operation',
					http_method != '', 'http_method',
					response_status_code != '', 'http_status',
					status_code_string != '', 'status',
					'') as dim,
			multiIf(service != '', service,
					name != '', name,
					http_method != '', http_method,
					response_status_code != '', response_status_code,
					status_code_string != '', status_code_string,
					'') as value,
			count() as cnt
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID` + prewhereFP + `
		WHERE timestamp BETWEEN @start AND @end AND is_root = 1` + where + `
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
	resourceWhere, where, args := filter.BuildClauses(req.Filters)
	grainSQL := timebucket.DisplayGrainSQL(req.EndTime - req.StartTime)

	var query string
	if resourceWhere == "" {
		query = `
			SELECT ` + grainSQL + `                          AS time_bucket,
			       countIf(is_error = 0)                     AS total,
			       countIf(is_error = 1)                     AS errors
			FROM optikk.spans
			PREWHERE tenant_id = @tenantID
			WHERE timestamp BETWEEN @start AND @end AND is_root = 1` + where + `
			GROUP BY time_bucket
			ORDER BY time_bucket ASC`
	} else {
		query = `
			WITH active_fps AS (
			    SELECT DISTINCT fingerprint
			    FROM optikk.spans_resource
			    PREWHERE tenant_id = @tenantID` + resourceWhere + `
			)
			SELECT ` + grainSQL + `                          AS time_bucket,
			       countIf(is_error = 0)                     AS total,
			       countIf(is_error = 1)                     AS errors
			FROM optikk.spans
			PREWHERE tenant_id = @tenantID AND fingerprint IN active_fps
			WHERE timestamp BETWEEN @start AND @end AND is_root = 1` + where + `
			GROUP BY time_bucket
			ORDER BY time_bucket ASC`
	}

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
		PREWHERE tenant_id = @tenantID
		WHERE timestamp BETWEEN @startMs AND @endMs
		  AND ` + column + ` != ''
		  AND (length(@prefix) = 0 OR positionCaseInsensitive(value, @prefix) > 0)
		GROUP BY value
		ORDER BY count DESC
		LIMIT @limit`
	var rows []suggestionRow
	if err := dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "suggest.SuggestScalar", &rows, query, suggestArgs(tenantID, startMs, endMs, prefix, limit)...); err != nil {
		return nil, err
	}
	out := make([]Suggestion, len(rows))
	for i, row := range rows {
		out[i] = Suggestion{Value: row.Value, Count: row.Count}
	}
	return out, nil
}

func (r *Repository) SuggestAttribute(ctx context.Context, tenantID, startMs, endMs int64, attrKey, prefix string, limit int) ([]Suggestion, error) {
	const query = `
		SELECT attributes[@attrKey]::String AS value, count() AS count
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		WHERE timestamp BETWEEN @startMs AND @endMs
		  AND value != ''
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
	out := make([]Suggestion, len(rows))
	for i, row := range rows {
		out[i] = Suggestion{Value: row.Value, Count: row.Count}
	}
	return out, nil
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

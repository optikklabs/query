package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/shared/chargs"
)

type PatternRaw struct {
	QueryHash      string  `ch:"query_hash"`
	QueryText      string  `ch:"query_text"`
	DBSystem       string  `ch:"db_system"`
	CollectionName string  `ch:"collection_name"`
	Namespace      string  `ch:"namespace"`
	Server         string  `ch:"server"`
	P50Ms          float32 `ch:"p50_ms"`
	P95Ms          float32 `ch:"p95_ms"`
	P99Ms          float32 `ch:"p99_ms"`
	CallCount      uint64  `ch:"call_count"`
	ErrorCount     uint64  `ch:"error_count"`
}

const DefaultPatternLimit = 20

const maxPatternLimit = 200

func clampPatternLimit(limit int) int {
	if limit <= 0 {
		return DefaultPatternLimit
	}
	return min(limit, maxPatternLimit)
}

func (r *Repository) GetSlowQueryPatterns(ctx context.Context, tenantID, startMs, endMs int64, f filter.Filters, limit int) ([]PatternRaw, error) {
	limit = clampPatternLimit(limit)
	filterWhere, filterArgs := filter.BuildSpanClauses(f)
	query := slowQueryPatternsQuery(filterWhere)
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), clickhouse.Named("qLimit", uint64(limit)))
	args = append(args, filterArgs...)
	var rows []PatternRaw
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "slowqueries.GetSlowQueryPatterns", &rows, query, args...)
}

func slowQueryPatternsQuery(filterWhere string) string {
	return `
		SELECT query_hash,
		       query_text,
		       db_system,
		       collection_name,
		       namespace,
		       server,
		       qs[1]      AS p50_ms,
		       qs[2]      AS p95_ms,
		       qs[3]      AS p99_ms,
		       call_count,
		       error_count
		FROM (
		    SELECT query_hash,
		           any(db_statement_normalized)                              AS query_text,
		           db_system,
		           db_name                                                    AS collection_name,
		           ''                                                         AS namespace,
		           ''                                                         AS server,
		           quantilesTiming(0.5, 0.95, 0.99)(duration_nano / 1000000.0) AS qs,
		           count()                                                   AS call_count,
		           countIf(is_error)                                         AS error_count
		    FROM optikk.spans
		    PREWHERE tenant_id = @tenantID
		         AND timestamp BETWEEN @start AND @end
		         AND db_system != ''
		         AND query_hash != ''` + filterWhere + `
		    GROUP BY query_hash, db_system, collection_name
		)
		ORDER BY call_count DESC
		LIMIT @qLimit`
}

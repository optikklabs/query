// Package slowqueries serves the slow-query panels by querying spans.
package slowqueries

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

type patternRawDTO struct {
	QueryHash      string  `ch:"query_hash"`
	QueryText      string  `ch:"query_text"`
	CollectionName string  `ch:"collection_name"`
	P50Ms          float32 `ch:"p50_ms"`
	P95Ms          float32 `ch:"p95_ms"`
	P99Ms          float32 `ch:"p99_ms"`
	CallCount      uint64  `ch:"call_count"`
	ErrorCount     uint64  `ch:"error_count"`
}

func (r *Repository) GetSlowQueryPatterns(ctx context.Context, teamID, startMs, endMs int64, f filter.Filters, limit int) ([]patternRawDTO, error) {
	if limit <= 0 {
		limit = 10
	}
	filterWhere, filterArgs := filter.BuildSpanClauses(f)
	query := `
		SELECT query_hash,
		       query_text,
		       collection_name,
		       qs[1]      AS p50_ms,
		       qs[2]      AS p95_ms,
		       qs[3]      AS p99_ms,
		       call_count,
		       error_count
		FROM (
		    SELECT query_hash,
		           any(db_statement_normalized)                          AS query_text,
		           attributes.'db.collection.name'::String               AS collection_name,
		           quantilesTiming(0.5, 0.95, 0.99)(duration_nano / 1000000.0) AS qs,
		           count()                                               AS call_count,
		           countIf(is_error)                                     AS error_count
		    FROM optikk.spans
		    PREWHERE team_id = @teamID
		         AND db_system != ''
		         AND db_statement != ''
		    WHERE timestamp BETWEEN @start AND @end` + filterWhere + `
		    GROUP BY query_hash, collection_name
		)
		ORDER BY call_count DESC
		LIMIT @qLimit`

	args := append(filter.SpanArgs(teamID, startMs, endMs), clickhouse.Named("qLimit", uint64(limit)))
	args = append(args, filterArgs...)
	var rows []patternRawDTO
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "slowqueries.GetSlowQueryPatterns", &rows, query, args...)
}

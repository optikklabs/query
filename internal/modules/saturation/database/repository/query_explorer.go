package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

type PatternRaw struct {
	QueryHash      string    `ch:"query_hash"`
	QueryText      string    `ch:"query_text"`
	DBSystem       string    `ch:"db_system"`
	CollectionName string    `ch:"collection_name"`
	Namespace      string    `ch:"namespace"`
	Server         string    `ch:"server"`
	QS             []float32 `ch:"qs"`
	CallCount      uint64    `ch:"call_count"`
	ErrorCount     uint64    `ch:"error_count"`
}

type QueryPatternsCursor struct {
	CallCount      uint64 `json:"callCount"`
	QueryHash      string `json:"queryHash"`
	DBSystem       string `json:"dbSystem"`
	CollectionName string `json:"collectionName"`
}

const DefaultPatternLimit = 20

func (r *Repository) QueryPatterns(
	ctx context.Context,
	tenantID, startMs, endMs int64,
	f filter.ExplorerFilters,
	limit int,
	cursor QueryPatternsCursor,
) ([]PatternRaw, error) {
	where, having, args := buildExplorerClauses(f, cursor)
	args = append(chargs.RangeArgs(tenantID, startMs, endMs), args...)
	args = append(args, clickhouse.Named("qLimit", uint64(limit)))

	query := `
		SELECT query_hash,
		       argMax(db_statement_normalized, (timestamp, span_id))       AS query_text,
		       db_system,
		       db_name                                                     AS collection_name,
		       ''                                                          AS namespace,
		       ''                                                          AS server,
		       quantilesTiming(0.5, 0.95, 0.99)(duration_nano / 1000000.0) AS qs,
		       count()                                                     AS call_count,
		       countIf(is_error)                                           AS error_count
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND db_system != ''
		     AND query_hash != ''
		WHERE 1=1` + where + `
		GROUP BY query_hash, db_system, collection_name` + having + `
		ORDER BY call_count DESC, query_hash ASC, db_system ASC, collection_name ASC
		LIMIT @qLimit`

	var rows []PatternRaw
	return rows, dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "database.QueryPatterns", &rows, query, args...)
}

func buildExplorerClauses(f filter.ExplorerFilters, cursor QueryPatternsCursor) (string, string, []any) {
	var where, having string
	var args []any
	args = filterutil.AppendIn(&where, args,
		filterutil.InClause{Column: "db_system", Bind: "dbSystems", Values: f.DBSystems},
		filterutil.InClause{Column: "db_name", Bind: "collections", Values: f.Collections},
		filterutil.InClause{Column: "service", Bind: "services", Values: f.Services},
	)
	if f.QueryText != "" {
		where += " AND positionCaseInsensitive(db_statement_normalized, @queryText) > 0"
		args = append(args, clickhouse.Named("queryText", f.QueryText))
	}

	addHaving := func(clause string, arg any) {
		having += " AND " + clause
		args = append(args, arg)
	}
	if f.MinCallCount != nil {
		addHaving("call_count >= @minCallCount", clickhouse.Named("minCallCount", *f.MinCallCount))
	}
	if f.MaxCallCount != nil {
		addHaving("call_count <= @maxCallCount", clickhouse.Named("maxCallCount", *f.MaxCallCount))
	}
	if f.MinErrorCount != nil {
		addHaving("error_count >= @minErrorCount", clickhouse.Named("minErrorCount", *f.MinErrorCount))
	}
	if f.MaxErrorCount != nil {
		addHaving("error_count <= @maxErrorCount", clickhouse.Named("maxErrorCount", *f.MaxErrorCount))
	}
	latencyBounds := []struct {
		value *float64
		alias string
		op    string
		bind  string
	}{
		{f.MinP50Ms, "qs[1]", ">=", "minP50Ms"},
		{f.MaxP50Ms, "qs[1]", "<=", "maxP50Ms"},
		{f.MinP95Ms, "qs[2]", ">=", "minP95Ms"},
		{f.MaxP95Ms, "qs[2]", "<=", "maxP95Ms"},
		{f.MinP99Ms, "qs[3]", ">=", "minP99Ms"},
		{f.MaxP99Ms, "qs[3]", "<=", "maxP99Ms"},
	}
	for _, bound := range latencyBounds {
		if bound.value != nil {
			addHaving(bound.alias+" "+bound.op+" @"+bound.bind, clickhouse.Named(bound.bind, *bound.value))
		}
	}
	if cursor.QueryHash != "" {
		having += ` AND (
			call_count < @cursorCallCount OR (
				call_count = @cursorCallCount AND (
					query_hash > @cursorQueryHash OR (
						query_hash = @cursorQueryHash AND (
							db_system > @cursorDBSystem OR (
								db_system = @cursorDBSystem AND collection_name > @cursorCollection
							)
						)
					)
				)
			)
		)`
		args = append(args,
			clickhouse.Named("cursorCallCount", cursor.CallCount),
			clickhouse.Named("cursorQueryHash", cursor.QueryHash),
			clickhouse.Named("cursorDBSystem", cursor.DBSystem),
			clickhouse.Named("cursorCollection", cursor.CollectionName),
		)
	}
	if having != "" {
		having = " HAVING 1=1" + having
	}
	return where, having, args
}

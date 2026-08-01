package repository

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
)

const (
	MaxCatalogueQueries = 200
	DefaultSeriesLimit  = 10
	MaxSeriesLimit      = 100
)

type CollectionOptionRaw struct {
	Name       string    `ch:"name"`
	QueryCount uint64    `ch:"query_count"`
	CallCount  uint64    `ch:"call_count"`
	QS         []float32 `ch:"qs"`
}

type QueryOptionRaw struct {
	QueryHash      string    `ch:"query_hash"`
	QueryLabel     string    `ch:"query_label"`
	CollectionName string    `ch:"collection_name"`
	CallCount      uint64    `ch:"call_count"`
	QS             []float32 `ch:"qs"`
	TotalQueries   uint64    `ch:"total_queries"`
}

type RankedQueryRaw struct {
	QueryHash      string `ch:"query_hash"`
	QueryLabel     string `ch:"query_label"`
	CollectionName string `ch:"collection_name"`
	CallCount      uint64 `ch:"call_count"`
}

type QuerySeriesPointRaw struct {
	BucketAt  time.Time `ch:"bucket_at"`
	QueryHash string    `ch:"query_hash"`
	QS        []float32 `ch:"qs"`
	OpsPerSec float64   `ch:"ops_per_sec"`
}

func (r *Repository) GetQueryCatalogue(
	ctx context.Context,
	tenantID, startMs, endMs int64,
	dbSystem string,
) ([]CollectionOptionRaw, []QueryOptionRaw, error) {
	table := timebucket.SpanStatsRollup(startMs, endMs)
	baseArgs := append(chargs.RangeArgs(tenantID, startMs, endMs), clickhouse.Named("dbSystem", dbSystem))
	collectionsQuery := `
		SELECT db_name AS name,
		       uniqExact(query_hash) AS query_count,
		       sum(request_count) AS call_count,
		       arrayMap(x -> toFloat32(x), quantilesTDigestMerge(0.95, 0.99)(latency_state)) AS qs
		FROM ` + table + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		WHERE db_system = @dbSystem AND db_name != '' AND query_hash != ''
		GROUP BY db_name
		ORDER BY call_count DESC, name ASC`
	var collections []CollectionOptionRaw
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "queryperformance.GetCollections", &collections, collectionsQuery, baseArgs...); err != nil {
		return nil, nil, err
	}

	queriesQuery := `
		SELECT query_hash,
		       argMax(span_name, timestamp) AS query_label,
		       argMax(db_name, timestamp) AS collection_name,
		       sum(request_count) AS call_count,
		       arrayMap(x -> toFloat32(x), quantilesTDigestMerge(0.95, 0.99)(latency_state)) AS qs,
		       count() OVER () AS total_queries
		FROM ` + table + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		WHERE db_system = @dbSystem AND db_name != '' AND query_hash != ''
		GROUP BY query_hash
		ORDER BY call_count DESC, query_hash ASC
		LIMIT @queryLimit`
	args := append(baseArgs, clickhouse.Named("queryLimit", uint64(MaxCatalogueQueries)))
	var queries []QueryOptionRaw
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "queryperformance.GetQueries", &queries, queriesQuery, args...); err != nil {
		return nil, nil, err
	}
	return collections, queries, nil
}

func (r *Repository) GetRankedQueries(
	ctx context.Context,
	tenantID, startMs, endMs int64,
	dbSystem, collection, queryHash string,
	limit int,
) ([]RankedQueryRaw, bool, error) {
	table := timebucket.SpanStatsRollup(startMs, endMs)
	scopeWhere := " AND db_name = @collection"
	scopeArg := clickhouse.Named("collection", collection)
	if queryHash != "" {
		scopeWhere = " AND query_hash = @queryHash"
		scopeArg = clickhouse.Named("queryHash", queryHash)
		limit = 1
	}
	query := `
		SELECT query_hash,
		       argMax(span_name, timestamp) AS query_label,
		       argMax(db_name, timestamp) AS collection_name,
		       sum(request_count) AS call_count
		FROM ` + table + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		WHERE db_system = @dbSystem AND query_hash != ''` + scopeWhere + `
		GROUP BY query_hash
		ORDER BY call_count DESC, query_hash ASC
		LIMIT @queryLimit`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("dbSystem", dbSystem),
		scopeArg,
		clickhouse.Named("queryLimit", uint64(limit+1)),
	)
	var rows []RankedQueryRaw
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "queryperformance.GetRankedQueries", &rows, query, args...); err != nil {
		return nil, false, err
	}
	truncated := len(rows) > limit
	if truncated {
		rows = rows[:limit]
	}
	return rows, truncated, nil
}

func (r *Repository) GetQuerySeriesPoints(
	ctx context.Context,
	tenantID, startMs, endMs int64,
	dbSystem, collection string,
	queryHashes []string,
) ([]QuerySeriesPointRaw, error) {
	if len(queryHashes) == 0 {
		return []QuerySeriesPointRaw{}, nil
	}
	collectionWhere := ""
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("dbSystem", dbSystem),
		clickhouse.Named("queryHashes", queryHashes),
	)
	if collection != "" {
		collectionWhere = " AND db_name = @collection"
		args = append(args, clickhouse.Named("collection", collection))
	}
	query := `
		SELECT ` + timebucket.DisplayGrainSQLForRange(startMs, endMs) + ` AS bucket_at,
		       query_hash,
		       arrayMap(x -> toFloat32(x), quantilesTDigestMerge(0.5, 0.95, 0.99)(latency_state)) AS qs,
		       sum(request_count) / @bucketGrainSec AS ops_per_sec
		FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		WHERE db_system = @dbSystem AND query_hash IN @queryHashes` + collectionWhere + `
		GROUP BY bucket_at, query_hash
		ORDER BY bucket_at, query_hash`
	args = timebucket.WithBucketGrainSec(args, startMs, endMs)
	var rows []QuerySeriesPointRaw
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "queryperformance.GetSeries", &rows, query, args...)
}

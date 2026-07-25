package topology

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// edgeWhere selects the span_stats rows that form graph edges: client-side
// spans whose resolved peer is a different service. Server-side rows carry no
// peer, so the same rollup serves both nodes and edges.
const edgeWhere = `kind_string IN ('CLIENT', 'PRODUCER')
		  AND peer_name != '' AND peer_name != service`

// GetNodes returns per-service RED aggregates and p50/p95/p99 latency.
func (r *Repository) GetNodes(ctx context.Context, tenantID, startMs, endMs int64, focusService string) ([]nodeAggRow, error) {
	rollup := timebucket.SpanStatsRollup(endMs - startMs)
	query := `
		WITH neighbor_services AS (
		    SELECT if(peer_name = @focusService, service, peer_name) AS service
		    FROM ` + rollup + `
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		    WHERE ` + edgeWhere + `
		      AND (service = @focusService OR peer_name = @focusService)
		)
		SELECT service                                            AS service,
		       sum(request_count)                                 AS request_count,
		       sumIf(request_count, ` + spanstats.ErrorPred + `)  AS error_count,
		       quantilesTDigestMerge(0.5, 0.95, 0.99)(latency_state) AS qs
		FROM ` + rollup + `
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		  AND service != ''
		  AND (@focusService = '' OR service = @focusService OR service IN (SELECT service FROM neighbor_services))
		GROUP BY service`
	var rows []nodeAggRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "topology.GetNodes",
		&rows, query, topologyArgs(tenantID, startMs, endMs, focusService)...); err != nil {
		return nil, err
	}
	for i := range rows {
		if len(rows[i].QS) >= 3 {
			rows[i].P50Ms = float32(rows[i].QS[0])
			rows[i].P95Ms = float32(rows[i].QS[1])
			rows[i].P99Ms = float32(rows[i].QS[2])
		}
	}
	return rows, nil
}

// GetEdges builds directed edges from the client side of each call: one row per
// (service -> peer) pair with call/error counts and latency as the caller
// observed it, so the numbers include network and queueing time.
func (r *Repository) GetEdges(ctx context.Context, tenantID, startMs, endMs int64, focusService string) ([]edgeAggRow, error) {
	query := `
		SELECT service                                           AS source,
		       peer_name                                         AS target,
		       sum(request_count)                                AS call_count,
		       sumIf(request_count, ` + spanstats.ErrorPred + `) AS error_count,
		       quantilesTDigestMerge(0.5, 0.95)(latency_state)   AS qs
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		WHERE ` + edgeWhere + `
		  AND (@focusService = '' OR service = @focusService OR peer_name = @focusService)
		GROUP BY source, target`
	var rows []edgeAggRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "topology.GetEdges",
		&rows, query, topologyArgs(tenantID, startMs, endMs, focusService)...); err != nil {
		return nil, err
	}
	for i := range rows {
		if len(rows[i].QS) >= 2 {
			rows[i].P50Ms = float32(rows[i].QS[0])
			rows[i].P95Ms = float32(rows[i].QS[1])
		}
	}
	return rows, nil
}

func topologyArgs(tenantID, startMs, endMs int64, focusService string) []any {
	return append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("focusService", focusService))
}

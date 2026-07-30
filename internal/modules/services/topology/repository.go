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

const edgeWhere = `kind_string IN ('CLIENT', 'PRODUCER')
		  AND peer_name != '' AND peer_name != service`

const maxTopologyNodes = 1000

// Nodes are returned fleet-wide; filterNeighborhood trims to the focus service.
func (r *Repository) GetNodes(ctx context.Context, tenantID, startMs, endMs int64) ([]nodeAggRow, error) {
	query := `
		SELECT service AS service_name,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.LatencyP50P95P99.SQL() + `
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		  AND service != ''
		GROUP BY service_name
		ORDER BY ` + spanstats.RequestTotal + ` DESC
		LIMIT @nodeLimit`
	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("nodeLimit", uint64(maxTopologyNodes)))
	var rows []nodeAggRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "topology.GetNodes",
		&rows, query, args...); err != nil {
		return nil, err
	}
	for i := range rows {
		p50, p95, p99 := spanstats.LatencyP50P95P99.P50P95P99(rows[i].QS)
		rows[i].P50Ms, rows[i].P95Ms, rows[i].P99Ms = float32(p50), float32(p95), float32(p99)
	}
	return rows, nil
}

func (r *Repository) GetEdges(ctx context.Context, tenantID, startMs, endMs int64, focusService string) ([]edgeAggRow, error) {
	query := `
		SELECT service   AS source,
		       peer_name AS target,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.LatencyP50P95.SQL() + `
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
		p50, p95 := spanstats.LatencyP50P95.P50P95(rows[i].QS)
		rows[i].P50Ms, rows[i].P95Ms = float32(p50), float32(p95)
	}
	return rows, nil
}

func topologyArgs(tenantID, startMs, endMs int64, focusService string) []any {
	return append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("focusService", focusService))
}

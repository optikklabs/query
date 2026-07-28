package repository

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type FleetPodAggregateRow struct {
	Pod           string    `ch:"pod"`
	Host          string    `ch:"host"`
	Services      []string  `ch:"services"`
	RequestCount  uint64    `ch:"request_total"`
	ErrorCount    uint64    `ch:"error_total"`
	DurationMsSum float64   `ch:"duration_ms_total"`
	P95LatencyMs  float32   `ch:"p95_latency_ms"`
	LastSeen      time.Time `ch:"last_seen"`
}

func (r *Repository) QueryFleetPods(ctx context.Context, tenantID int64, startMs, endMs int64, host string) ([]FleetPodAggregateRow, error) {
	query := `
		SELECT
		    pod                                                      AS pod,
		    if(host != '', host, @defaultUnknown)                    AS host,
		    groupUniqArray(service)                                  AS services,
		    ` + spanstats.Requests + `,
		    ` + spanstats.Errors + `,
		    ` + spanstats.DurationSum + `,
		    toFloat32(quantilesTDigestMerge(0.95)(latency_state)[1]) AS p95_latency_ms,
		    max(timestamp)                                           AS last_seen
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		WHERE pod != '' AND (@hostFilter = '' OR host = @hostFilter)
		GROUP BY pod, host
		ORDER BY ` + spanstats.RequestTotal + ` DESC
		LIMIT @maxFleetPods`
	args := chargs.RollupRangeArgs(tenantID, startMs, endMs)
	args = append(args,
		clickhouse.Named("defaultUnknown", unknownHost),
		clickhouse.Named("maxFleetPods", uint64(maxFleetPods)),
		clickhouse.Named("hostFilter", host),
	)
	var rows []FleetPodAggregateRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "fleet.QueryFleetPods", &rows, query, args...)
}

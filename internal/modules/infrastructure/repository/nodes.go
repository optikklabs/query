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

type NodeAggregateRow struct {
	Host          string    `ch:"host"`
	PodCount      uint64    `ch:"pod_count"`
	RequestCount  uint64    `ch:"request_total"`
	ErrorCount    uint64    `ch:"error_total"`
	DurationMsSum float64   `ch:"duration_ms_total"`
	P95LatencyMs  float32   `ch:"p95_latency_ms"`
	LastSeen      time.Time `ch:"last_seen"`
}

type NodeServiceAggregateRow struct {
	Service       string  `ch:"service"`
	RequestCount  uint64  `ch:"request_total"`
	ErrorCount    uint64  `ch:"error_total"`
	DurationMsSum float64 `ch:"duration_ms_total"`
	P95LatencyMs  float32 `ch:"p95_latency_ms"`
	PodCount      uint64  `ch:"pod_count"`
}

type NodeSummaryRow struct {
	HealthyNodes   uint64  `ch:"healthy_nodes"`
	DegradedNodes  uint64  `ch:"degraded_nodes"`
	UnhealthyNodes uint64  `ch:"unhealthy_nodes"`
	TotalPods      *uint64 `ch:"total_pods"`
}

func (r *Repository) QueryInfrastructureNodes(ctx context.Context, tenantID int64, startMs, endMs int64) ([]NodeAggregateRow, error) {
	query := `
		SELECT
		    if(host != '', host, @defaultUnknown)                    AS host,
		    uniqExactIf(pod, pod != '')                              AS pod_count,
		    ` + spanstats.Requests + `,
		    ` + spanstats.Errors + `,
		    ` + spanstats.DurationSum + `,
		    toFloat32(quantilesTDigestMerge(0.95)(latency_state)[1]) AS p95_latency_ms,
		    max(timestamp)                                           AS last_seen
		FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		GROUP BY host
		ORDER BY ` + spanstats.RequestTotal + ` DESC, host ASC
		LIMIT @maxNodes`
	args := chargs.RangeArgs(tenantID, startMs, endMs)
	args = append(args,
		clickhouse.Named("defaultUnknown", unknownHost),
		clickhouse.Named("maxNodes", uint64(MaxNodes)),
	)
	var rows []NodeAggregateRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "nodes.QueryInfrastructureNodes", &rows, query, args...)
}

func (r *Repository) QueryInfrastructureNodeSummary(ctx context.Context, tenantID int64, startMs, endMs int64) (NodeSummaryRow, error) {
	query := `
		SELECT countIf(error_rate <= 2)                    AS healthy_nodes,
		       countIf(error_rate > 2 AND error_rate <= 10) AS degraded_nodes,
		       countIf(error_rate > 10)                    AS unhealthy_nodes,
		       sum(pod_count)                              AS total_pods
		FROM (
		    SELECT if(sum(request_count) = 0, 0,
		              100.0 * sumIf(request_count, ` + spanstats.ErrorPred + `) / sum(request_count)) AS error_rate,
		           uniqExactIf(pod, pod != '') AS pod_count
		    FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + `
		    PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		    GROUP BY if(host != '', host, @defaultUnknown)
		)`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("defaultUnknown", unknownHost))
	var row NodeSummaryRow
	return row, dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "nodes.QueryInfrastructureNodeSummary", &row, query, args...)
}

func (r *Repository) QueryInfrastructureNodeServices(ctx context.Context, tenantID int64, host string, startMs, endMs int64) ([]NodeServiceAggregateRow, error) {
	query := `
		SELECT
		    service                                                  AS service,
		    ` + spanstats.Requests + `,
		    ` + spanstats.Errors + `,
		    ` + spanstats.DurationSum + `,
		    toFloat32(quantilesTDigestMerge(0.95)(latency_state)[1]) AS p95_latency_ms,
		    uniqExactIf(pod, pod != '')                              AS pod_count
		FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		WHERE if(host != '', host, @defaultUnknown) = @host
		GROUP BY service
		ORDER BY ` + spanstats.RequestTotal + ` DESC, service ASC
		LIMIT @maxServices`
	args := chargs.RangeArgs(tenantID, startMs, endMs)
	args = append(args,
		clickhouse.Named("host", host),
		clickhouse.Named("defaultUnknown", unknownHost),
		clickhouse.Named("maxServices", uint64(MaxServices)),
	)
	var rows []NodeServiceAggregateRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "nodes.QueryInfrastructureNodeServices", &rows, query, args...)
}

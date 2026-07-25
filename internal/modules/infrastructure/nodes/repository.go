package nodes

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/metrics"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

// Repository reads the span_stats rollup for per-host RED.

// Query limits and defaults for node aggregates.
const (
	MaxNodes       = 200
	MaxServices    = 100
	DefaultUnknown = "unknown"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) QueryInfrastructureNodes(ctx context.Context, tenantID int64, startMs, endMs int64) ([]NodeAggregateRow, error) {
	query := `
		SELECT
		    if(host != '', host, @defaultUnknown)                    AS host,
		    uniqIf(pod, pod != '')                                   AS pod_count,
		    ` + spanstats.Requests + `,
		    ` + spanstats.Errors + `,
		    ` + spanstats.DurationSum + `,
		    toFloat32(quantilesTDigestMerge(0.95)(latency_state)[1]) AS p95_latency_ms,
		    max(timestamp)                                           AS last_seen
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		GROUP BY host
		ORDER BY ` + spanstats.RequestTotal + ` DESC
		LIMIT @maxNodes`
	args := chargs.RollupRangeArgs(tenantID, startMs, endMs)
	args = append(args,
		clickhouse.Named("defaultUnknown", DefaultUnknown),
		clickhouse.Named("maxNodes", uint64(MaxNodes)),
	)
	var rows []NodeAggregateRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "nodes.QueryInfrastructureNodes", &rows, query, args...)
}

func (r *Repository) QueryInfrastructureNodeSummary(ctx context.Context, tenantID int64, startMs, endMs int64) (NodeSummaryRow, error) {
	nodeRows, err := r.QueryInfrastructureNodes(ctx, tenantID, startMs, endMs)
	if err != nil {
		return NodeSummaryRow{}, err
	}

	var healthy, degraded, unhealthy, totalPods uint64
	for _, raw := range nodeRows {
		totalPods += raw.PodCount
		errorRate := metrics.Percentage(raw.ErrorCount, raw.RequestCount)
		switch {
		case errorRate > 10:
			unhealthy++
		case errorRate > 2 && errorRate <= 10:
			degraded++
		default:
			healthy++
		}
	}

	return NodeSummaryRow{
		HealthyNodes:   healthy,
		DegradedNodes:  degraded,
		UnhealthyNodes: unhealthy,
		TotalPods:      &totalPods,
	}, nil
}

func (r *Repository) QueryInfrastructureNodeServices(ctx context.Context, tenantID int64, host string, startMs, endMs int64) ([]NodeServiceAggregateRow, error) {
	query := `
		SELECT
		    service                                                  AS service,
		    ` + spanstats.Requests + `,
		    ` + spanstats.Errors + `,
		    ` + spanstats.DurationSum + `,
		    toFloat32(quantilesTDigestMerge(0.95)(latency_state)[1]) AS p95_latency_ms,
		    uniqIf(pod, pod != '')                                   AS pod_count
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		WHERE if(host != '', host, @defaultUnknown) = @host
		GROUP BY service
		ORDER BY ` + spanstats.RequestTotal + ` DESC
		LIMIT @maxServices`
	args := chargs.RollupRangeArgs(tenantID, startMs, endMs)
	args = append(args,
		clickhouse.Named("host", host),
		clickhouse.Named("defaultUnknown", DefaultUnknown),
		clickhouse.Named("maxServices", uint64(MaxServices)),
	)
	var rows []NodeServiceAggregateRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "nodes.QueryInfrastructureNodeServices", &rows, query, args...)
}

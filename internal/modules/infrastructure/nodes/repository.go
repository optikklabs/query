package nodes

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/metrics"
	"github.com/optikklabs/query/internal/shared/seriesattr"
)

// Repository reads spanmetrics (traces.span.metrics.duration) for per-host RED.

// Query limits and defaults for node aggregates.
const (
	MaxNodes       = 200
	MaxServices    = 100
	DefaultUnknown = "unknown"
)

const hostSeriesCTE = `
		WITH series AS (
		    SELECT fingerprint,
		           host,
		           pod,
		           service,
		           ` + seriesattr.StatusCode + ` AS status_code
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    GROUP BY fingerprint, host, pod, service, status_code
		)`

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) QueryInfrastructureNodes(ctx context.Context, tenantID int64, startMs, endMs int64) ([]NodeAggregateRow, error) {
	query := hostSeriesCTE + `
		SELECT
		    if(series.host != '', series.host, @defaultUnknown)                   AS host,
		    uniqIf(series.pod, series.pod != '')                                  AS pod_count,
		    sum(m.hist_count)                                                     AS request_count,
		    sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)               AS error_count,
		    sum(m.hist_sum)                                                       AS duration_ms_sum,
		    toFloat32(quantilesPrometheusHistogramMerge(0.95)(m.latency_state)[1]) AS p95_latency_ms,
		    max(m.timestamp)                                                      AS last_seen
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY host
		ORDER BY request_count DESC
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
	query := hostSeriesCTE + `
		SELECT
		    series.host                                            AS host,
		    sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `) AS error_count,
		    sum(m.hist_count)                                      AS request_count,
		    uniqIf(series.pod, series.pod != '')                  AS pod_count
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY host`
	args := chargs.RollupRangeArgs(tenantID, startMs, endMs)
	type nodeRawSummaryRow struct {
		Host         string `ch:"host"`
		ErrorCount   uint64 `ch:"error_count"`
		RequestCount uint64 `ch:"request_count"`
		PodCount     uint64 `ch:"pod_count"`
	}
	var rawRows []nodeRawSummaryRow
	if err := dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "nodes.QueryInfrastructureNodeSummary", &rawRows, query, args...); err != nil {
		return NodeSummaryRow{}, err
	}

	var healthy, degraded, unhealthy, totalPods uint64
	for _, raw := range rawRows {
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
	query := hostSeriesCTE + `
		SELECT
		    series.service                                                       AS service,
		    sum(m.hist_count)                                                     AS request_count,
		    sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)               AS error_count,
		    sum(m.hist_sum)                                                       AS duration_ms_sum,
		    toFloat32(quantilesPrometheusHistogramMerge(0.95)(m.latency_state)[1]) AS p95_latency_ms,
		    uniqIf(series.pod, series.pod != '')                                 AS pod_count
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		WHERE if(series.host != '', series.host, @defaultUnknown) = @host
		GROUP BY service
		ORDER BY request_count DESC
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

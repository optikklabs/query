package hosts

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/seriesattr"
)

const defaultUnknownHost = "unknown"

// stateAttr resolves the hostmetrics per-state label (idle/user/used/...) from
// a metrics_series row, used to pick the meaningful utilization slice.
const stateAttr = "attributes.`state`::String"

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// QueryHostUtilization returns metric utilization for CPU, memory, and disk.
func (r *Repository) QueryHostUtilization(ctx context.Context, teamID, startMs, endMs int64) ([]hostMetricRow, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	// Host is grouped from metrics_series; the scalar rollup supplies values.
	// CPU/memory utilization arrive split per state (and per core), so a plain
	// mean over all datapoints is a 1/Nstates artifact. Keep cpu 'idle' (and
	// invert to busy below) and memory 'used'; other metrics are untouched.
	query := `
		WITH fps AS (
		    SELECT fingerprint, any(host) AS host
		    FROM optikk.metrics_series AS mr
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name IN @metricNames
		    WHERE mr.host != ''
		      AND NOT (metric_name = @cpuUtil AND ` + stateAttr + ` != 'idle')
		      AND NOT (metric_name = @memUtil AND ` + stateAttr + ` != 'used')
		    GROUP BY fingerprint
		)
		SELECT
		    r.host        AS host,
		    m.metric_name AS metric_name,
		    if(m.metric_name = @cpuUtil,
		       1 - sum(m.val_sum) / sum(m.val_count),
		       sum(m.val_sum) / sum(m.val_count)) AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN fps AS r ON m.fingerprint = r.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY host, metric_name`

	bucketStart, bucketEnd := chargs.BucketBounds(startMs, endMs)
	args := []any{
		clickhouse.Named("teamID", uint32(teamID)),
		clickhouse.Named("bucketStart", bucketStart),
		clickhouse.Named("bucketEnd", bucketEnd),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("metricNames", utilizationMetricNames()),
		clickhouse.Named("cpuUtil", infraconsts.MetricSystemCPUUtilization),
		clickhouse.Named("memUtil", infraconsts.MetricSystemMemoryUtilization),
	}
	var rows []hostMetricRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "hosts.QueryHostUtilization", &rows, query, args...)
}

// QueryHostSpans returns host RED aggregates from spanmetrics duration.
func (r *Repository) QueryHostSpans(
	ctx context.Context, teamID, startMs, endMs int64, serviceName string,
) ([]hostSpansRow, error) {
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           any(host)                          AS host,
		           any(environment)                   AS environment,
		           any(` + seriesattr.StatusCode + `) AS status_code
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE service = @serviceName
		    GROUP BY fingerprint
		)
		SELECT
		    if(series.host != '', series.host, @unknownHost)        AS host,
		    any(series.environment)                                 AS zone,
		    sum(m.hist_count)                                        AS request_count,
		    sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `) AS error_count,
		    toFloat32(quantilesPrometheusHistogramMerge(0.99)(m.latency_state)[1]) AS p99_ms,
		    max(m.timestamp)                                        AS last_seen
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY host
		ORDER BY request_count DESC`
	args := append(chargs.RollupRangeArgs(teamID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("unknownHost", defaultUnknownHost),
	)
	var rows []hostSpansRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "hosts.QueryHostSpans",
		&rows, query, args...)
}

// utilizationMetricNames returns the CPU, memory, and disk metric names.
func utilizationMetricNames() []string {
	names := make([]string, 0, len(infraconsts.CPUMetrics)+len(infraconsts.MemoryMetrics)+len(infraconsts.DiskMetrics))
	names = append(names, infraconsts.CPUMetrics...)
	names = append(names, infraconsts.MemoryMetrics...)
	names = append(names, infraconsts.DiskMetrics...)
	return names
}

package hosts

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

const defaultUnknownHost = "unknown"

// stateAttr resolves the hostmetrics per-state label (idle/user/used/...) from
// a metrics_series row, used to pick the meaningful utilization slice.
const stateAttr = "attributes['state']"

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) QueryHostUtilization(ctx context.Context, tenantID, startMs, endMs int64) ([]hostMetricRow, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)

	query := `
		WITH fps AS (
		    SELECT fingerprint, host
		    FROM optikk.metrics_series AS mr
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name IN @metricNames
		    WHERE mr.host != ''
		      AND NOT (metric_name = @cpuUtil AND ` + stateAttr + ` != 'idle')
		      AND NOT (metric_name = @memUtil AND ` + stateAttr + ` != 'used')
		    GROUP BY fingerprint, host
		)
		SELECT
		    r.host        AS host,
		    m.metric_name AS metric_name,
		    if(m.metric_name = @cpuUtil,
		       1 - sum(m.val_sum) / sum(m.val_count),
		       sum(m.val_sum) / sum(m.val_count)) AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN fps AS r ON m.fingerprint = r.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY host, metric_name
		ORDER BY host, metric_name
		LIMIT 500`

	args := []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
		clickhouse.Named("metricNames", utilizationMetricNames()),
		clickhouse.Named("cpuUtil", infraconsts.MetricSystemCPUUtilization),
		clickhouse.Named("memUtil", infraconsts.MetricSystemMemoryUtilization),
	}
	var rows []hostMetricRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "hosts.QueryHostUtilization", &rows, query, args...)
}

func (r *Repository) QueryHostSpans(
	ctx context.Context, tenantID, startMs, endMs int64, serviceName string,
) ([]hostSpansRow, error) {
	query := `
		SELECT
		    if(host != '', host, @unknownHost)                       AS host,
		    any(environment)                                         AS zone,
		    sum(request_count)                                       AS request_count,
		    sumIf(request_count, ` + spanstats.ErrorPred + `)        AS error_count,
		    toFloat32(quantileTDigestMerge(0.99)(latency_state))     AS p99_ms,
		    max(timestamp)                                           AS last_seen
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND service = @serviceName
		GROUP BY host
		ORDER BY request_count DESC
		LIMIT 200`
	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("unknownHost", defaultUnknownHost),
	)
	var rows []hostSpansRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "hosts.QueryHostSpans",
		&rows, query, args...)
}

func utilizationMetricNames() []string {
	names := make([]string, 0, len(infraconsts.CPUMetrics)+len(infraconsts.MemoryMetrics)+len(infraconsts.DiskMetrics))
	names = append(names, infraconsts.CPUMetrics...)
	names = append(names, infraconsts.MemoryMetrics...)
	names = append(names, infraconsts.DiskMetrics...)
	return names
}

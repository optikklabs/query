package repository

import (
	"context"
	"slices"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type HostMetricRow struct {
	Host       string  `ch:"host"`
	MetricName string  `ch:"metric_name"`
	Value      float64 `ch:"value"`
}

type HostSpansRow struct {
	Host         string    `ch:"host"`
	Zone         string    `ch:"zone"`
	RequestCount uint64    `ch:"request_total"`
	ErrorCount   uint64    `ch:"error_total"`
	P99Ms        float64   `ch:"p99_ms"`
	LastSeen     time.Time `ch:"last_seen"`
}

func (r *Repository) QueryHostUtilization(ctx context.Context, tenantID, startMs, endMs int64) ([]HostMetricRow, error) {
	query := `
		SELECT
		    host,
		    metric_name,
		    ` + infraconsts.UsageValueSQL + ` AS value
		FROM ` + timebucket.MetricsRollup(startMs, endMs) + `
		PREWHERE tenant_id     = @tenantID
		     AND metric_name IN @metricNames
		     AND timestamp >= @start AND timestamp < @end
		     AND host != ''
		WHERE ` + infraconsts.UsageFilterSQL + `
		GROUP BY host, metric_name
		ORDER BY host, metric_name
		LIMIT 500`

	args := chargs.WithMetricNames(chargs.RangeArgs(tenantID, startMs, endMs), utilizationMetricNames)
	var rows []HostMetricRow
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "hosts.QueryHostUtilization", &rows, query, args...)
	return rows, err
}

func (r *Repository) QueryHostSpans(
	ctx context.Context, tenantID, startMs, endMs int64, serviceName string,
) ([]HostSpansRow, error) {
	query := `
		SELECT
		    if(host != '', host, @unknownHost)                       AS host,
		    argMax(environment, (timestamp, environment))            AS zone,
		    ` + spanstats.Requests + `,
		    ` + spanstats.Errors + `,
		    toFloat64(quantilesTDigestMerge(0.99)(latency_state)[1]) AS p99_ms,
		    max(timestamp)                                           AS last_seen
		FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND service = @serviceName
		GROUP BY host
		ORDER BY ` + spanstats.RequestTotal + ` DESC, host ASC
		LIMIT 200`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("unknownHost", unknownHost),
	)
	var rows []HostSpansRow
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "hosts.QueryHostSpans",
		&rows, query, args...)
	return rows, err
}

var utilizationMetricNames = slices.Concat(infraconsts.CPUMetrics, infraconsts.MemoryMetrics, infraconsts.DiskMetrics)

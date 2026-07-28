package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/shared/chargs"
)

var memMetricNames = []string{
	infraconsts.MetricSystemMemoryUtilization,
	infraconsts.MetricSystemMemoryUsage,
	infraconsts.MetricJVMMemoryUsed,
	infraconsts.MetricJVMMemoryMax,
}

type MemoryMetricNameRow struct {
	MetricName string  `ch:"metric_name"`
	Value      float64 `ch:"value"`
}

func (r *Repository) QueryMemoryUtilizationAgg(ctx context.Context, tenantID int64, startMs, endMs int64) ([]MemoryMetricNameRow, error) {
	query := `
		SELECT
		    metric_name AS metric_name,
		    sum(val_sum) / sum(val_count)  AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + `
		PREWHERE tenant_id        = @tenantID
		     AND metric_name   IN @metricNames
		     AND timestamp   BETWEEN @start AND @end
		GROUP BY metric_name`
	args := chargs.WithMetricNames(chargs.RollupRangeArgs(tenantID, startMs, endMs), memMetricNames)
	var rows []MemoryMetricNameRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "memory.QueryMemoryUtilizationAgg", &rows, query, args...)
}

func (r *Repository) QueryMemoryUtilizationForInstance(ctx context.Context, tenantID int64, startMs, endMs int64, host, pod, serviceName string) ([]MemoryMetricNameRow, error) {
	query := `
		WITH fps AS (
		    SELECT fingerprint
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name IN @metricNames
		    WHERE host = @host AND service = @serviceName AND pod = @pod
		)
		SELECT
		    metric_name AS metric_name,
		    sum(val_sum) / sum(val_count)  AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + `
		PREWHERE tenant_id        = @tenantID
		     AND metric_name   IN @metricNames
		     AND timestamp   BETWEEN @start AND @end
		WHERE fingerprint IN fps
		GROUP BY metric_name`
	args := chargs.WithMetricNames(chargs.RollupRangeArgs(tenantID, startMs, endMs), memMetricNames)
	args = append(args,
		clickhouse.Named("host", host),
		clickhouse.Named("pod", pod),
		clickhouse.Named("serviceName", serviceName),
	)
	var rows []MemoryMetricNameRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "memory.QueryMemoryUtilizationForInstance", &rows, query, args...)
}

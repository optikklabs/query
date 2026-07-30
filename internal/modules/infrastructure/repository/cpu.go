package repository

import (
	"context"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/shared/chargs"
)

type CPUMetricNameRow struct {
	MetricName string  `ch:"metric_name"`
	Value      float64 `ch:"value"`
}

type CPUInstanceMetricRow struct {
	Host       string  `ch:"host"`
	Pod        string  `ch:"pod"`
	Container  string  `ch:"container"`
	Service    string  `ch:"service"`
	MetricName string  `ch:"metric_name"`
	Value      float64 `ch:"value"`
}

func (r *Repository) QueryCPUUtilizationAgg(ctx context.Context, tenantID int64, startMs, endMs int64) ([]CPUMetricNameRow, error) {
	query := `
		SELECT
		    metric_name AS metric_name,
		    if(sum(val_count) = 0, 0, sum(val_sum) / sum(val_count))  AS value
		FROM ` + timebucket.MetricsRollup(startMs, endMs) + `
		PREWHERE tenant_id        = @tenantID
		     AND metric_name   IN @metricNames
		     AND timestamp >= @start AND timestamp < @end
		GROUP BY metric_name`
	args := chargs.WithMetricNames(chargs.RangeArgs(tenantID, startMs, endMs), infraconsts.CPUMetrics)
	var rows []CPUMetricNameRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "cpu.QueryCPUUtilizationAgg", &rows, query, args...)
}

func (r *Repository) QueryCPUUtilizationByInstance(ctx context.Context, tenantID int64, startMs, endMs int64) ([]CPUInstanceMetricRow, error) {

	query := `
		SELECT
		    host,
		    pod,
		    container,
		    service,
		    metric_name,
		    if(sum(val_count) = 0, 0, sum(val_sum) / sum(val_count)) AS value
		FROM ` + timebucket.MetricsRollup(startMs, endMs) + `
		PREWHERE tenant_id     = @tenantID
		     AND metric_name IN @metricNames
		     AND timestamp >= @start AND timestamp < @end
		     AND service != ''
		GROUP BY host, pod, container, service, metric_name
		ORDER BY service, pod
		LIMIT 500`
	args := chargs.WithMetricNames(chargs.RangeArgs(tenantID, startMs, endMs), infraconsts.CPUMetrics)
	var rows []CPUInstanceMetricRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "cpu.QueryCPUUtilizationByInstance", &rows, query, args...)
}

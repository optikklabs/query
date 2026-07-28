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
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + `
		PREWHERE tenant_id        = @tenantID
		     AND metric_name   IN @metricNames
		     AND timestamp   BETWEEN @start AND @end
		GROUP BY metric_name`
	args := chargs.WithMetricNames(chargs.RollupRangeArgs(tenantID, startMs, endMs), infraconsts.CPUMetrics)
	var rows []CPUMetricNameRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "cpu.QueryCPUUtilizationAgg", &rows, query, args...)
}

func (r *Repository) QueryCPUUtilizationByInstance(ctx context.Context, tenantID int64, startMs, endMs int64) ([]CPUInstanceMetricRow, error) {

	query := `
		WITH fps AS (
		    SELECT fingerprint,
		           host,
		           pod,
		           container,
		           service
		    FROM optikk.metrics_series AS mr
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name IN @metricNames
		    WHERE mr.service != ''
		    GROUP BY fingerprint, host, pod, container, service
		)
		SELECT
		    r.host                        AS host,
		    r.pod                         AS pod,
		    r.container                   AS container,
		    r.service                     AS service,
		    m.metric_name                 AS metric_name,
		    if(sum(m.val_count) = 0, 0, sum(m.val_sum) / sum(m.val_count)) AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN fps AS r ON m.fingerprint = r.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY host, pod, container, service, metric_name
		ORDER BY service, pod
		LIMIT 500`
	args := chargs.WithMetricNames(chargs.RollupRangeArgs(tenantID, startMs, endMs), infraconsts.CPUMetrics)
	var rows []CPUInstanceMetricRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "cpu.QueryCPUUtilizationByInstance", &rows, query, args...)
}

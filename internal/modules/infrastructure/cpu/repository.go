package cpu

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/shared/chargs"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) QueryCPUUtilizationAgg(ctx context.Context, teamID int64, startMs, endMs int64) ([]CPUMetricNameRow, error) {
	query := `
		SELECT
		    metric_name AS metric_name,
		    sum(val_sum) / sum(val_count)  AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + `
		PREWHERE team_id        = @teamID
		     AND metric_name   IN @metricNames
		     AND timestamp   BETWEEN @start AND @end
		GROUP BY metric_name`
	args := chargs.WithMetricNames(chargs.RollupRangeArgs(teamID, startMs, endMs), infraconsts.CPUMetrics)
	var rows []CPUMetricNameRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "cpu.QueryCPUUtilizationAgg", &rows, query, args...)
}

func (r *Repository) QueryCPUUtilizationByInstance(ctx context.Context, teamID int64, startMs, endMs int64) ([]CPUInstanceMetricRow, error) {
	// Resource dims (host/pod/container/service) live in metrics_series;
	// resolve them per fingerprint and join the scalar rollup on fingerprint.
	query := `
		WITH fps AS (
		    SELECT fingerprint,
		           host,
		           pod,
		           container,
		           service
		    FROM optikk.metrics_series AS mr
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name IN @metricNames
		    WHERE mr.service != ''
		    GROUP BY fingerprint, host, pod, container, service
		)
		SELECT
		    r.host                        AS host,
		    r.pod                         AS pod,
		    r.container                   AS container,
		    r.service                     AS service,
		    m.metric_name                 AS metric_name,
		    sum(m.val_sum) / sum(m.val_count) AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN fps AS r ON m.fingerprint = r.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY host, pod, container, service, metric_name
		ORDER BY service, pod`
	args := chargs.WithMetricNames(chargs.RollupRangeArgs(teamID, startMs, endMs), infraconsts.CPUMetrics)
	var rows []CPUInstanceMetricRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "cpu.QueryCPUUtilizationByInstance", &rows, query, args...)
}

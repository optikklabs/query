package fleet

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/seriesattr"
)

const (
	maxFleetPods   = 200
	defaultUnknown = "unknown"
)

// fleet reads spanmetrics duration for per-pod RED aggregates.

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) QueryFleetPods(ctx context.Context, tenantID int64, startMs, endMs int64, host string) ([]FleetPodAggregateRow, error) {
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           host,
		           pod,
		           service,
		           ` + seriesattr.StatusCode + ` AS status_code
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE metrics_series.pod != '' AND (@hostFilter = '' OR metrics_series.host = @hostFilter)
		    GROUP BY fingerprint, host, pod, service, status_code
		)
		SELECT
		    series.pod                                                            AS pod,
		    if(series.host != '', series.host, @defaultUnknown)                   AS host,
		    groupUniqArray(series.service)                                        AS services,
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
		GROUP BY pod, host
		ORDER BY request_count DESC
		LIMIT @maxFleetPods`
	args := chargs.RollupRangeArgs(tenantID, startMs, endMs)
	args = append(args,
		clickhouse.Named("defaultUnknown", defaultUnknown),
		clickhouse.Named("maxFleetPods", uint64(maxFleetPods)),
		clickhouse.Named("hostFilter", host),
	)
	var rows []FleetPodAggregateRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "fleet.QueryFleetPods", &rows, query, args...)
}

package containerdetail

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

// scopeColumn is the metrics_series column that scopes a pod's series.
const scopeColumn = "pod"

type Repository struct {
	db     clickhouse.Conn
	series *seriesgroup.Repository
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db, series: seriesgroup.NewRepository(db)}
}

// QuerySeries returns display-grain buckets for one metric group on a pod,
// broken down by the group's label (container/direction/interface/type).
func (r *Repository) QuerySeries(ctx context.Context, tenantID int64, pod string, startMs, endMs int64, def seriesgroup.Def) ([]SeriesPoint, error) {
	return r.series.QuerySeries(ctx, tenantID, scopeColumn, pod, startMs, endMs, def)
}

// QueryPodMeta returns identity metadata and which pod metrics the pod
// reports in range, from the narrow series-metadata table.
func (r *Repository) QueryPodMeta(ctx context.Context, tenantID int64, pod string, startMs, endMs int64) (podMetaRow, error) {
	query := `
		SELECT
		    max(timestamp)                                                        AS last_seen,
		    anyLastIf(host, host != '')                                           AS host,
		    groupUniqArrayIf(container, container != '')                          AS containers,
		    groupUniqArrayIf(service, service != '')                              AS services,
		    groupUniqArrayIf(environment, environment != '')                      AS environments,
		    groupUniqArrayIf(k8s_namespace, k8s_namespace != '')                  AS namespaces,
		    groupUniqArrayIf(metric_name, metric_name IN @podMetricNames)         AS metric_names
		FROM optikk.metrics_series
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		WHERE pod = @pod`
	args := chargs.RangeArgs(tenantID, startMs, endMs)
	args = append(args,
		clickhouse.Named("pod", pod),
		clickhouse.Named("podMetricNames", catalog.MetricNames()),
	)
	var row podMetaRow
	return row, dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "containerdetail.QueryPodMeta", &row, query, args...)
}

// QueryPodRED returns range RED aggregates for one pod from span metrics.
func (r *Repository) QueryPodRED(ctx context.Context, tenantID int64, pod string, startMs, endMs int64) (podREDRow, error) {
	query := `
		SELECT
		    sum(request_count)                                       AS request_count,
		    sumIf(request_count, ` + spanstats.ErrorPred + `)        AS error_count,
		    sum(duration_ms_sum)                                     AS duration_ms_sum,
		    toFloat32(quantilesTDigestMerge(0.95)(latency_state)[1]) AS p95_latency_ms
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND pod = @pod`
	args := chargs.RollupRangeArgs(tenantID, startMs, endMs)
	args = append(args,
		clickhouse.Named("pod", pod),
	)
	var row podREDRow
	return row, dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "containerdetail.QueryPodRED", &row, query, args...)
}

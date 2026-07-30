package repository

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/models"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesdefs"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type PodMetaRow struct {
	LastSeen     time.Time `ch:"last_seen"`
	Host         string    `ch:"host_any"`
	Containers   []string  `ch:"containers"`
	Services     []string  `ch:"services"`
	Environments []string  `ch:"environments"`
	Namespaces   []string  `ch:"namespaces"`
	MetricNames  []string  `ch:"metric_names"`
}

type PodREDRow struct {
	RequestCount  uint64  `ch:"request_total"`
	ErrorCount    uint64  `ch:"error_total"`
	DurationMsSum float64 `ch:"duration_ms_total"`
	P95LatencyMs  float32 `ch:"p95_latency_ms"`
}

func (r *Repository) QueryPodSeries(ctx context.Context, tenantID int64, pod string, startMs, endMs int64, def seriesgroup.Def) ([]models.SeriesPoint, error) {
	return r.series.QuerySeries(ctx, tenantID, podScopeColumn, pod, startMs, endMs, def)
}

func (r *Repository) QueryPodMeta(ctx context.Context, tenantID int64, pod string, startMs, endMs int64) (PodMetaRow, error) {
	query := `
		SELECT
		    max(timestamp)                                                        AS last_seen,
		    argMaxIf(host, (timestamp, fingerprint), host != '')                  AS host_any,
		    groupUniqArrayIf(container, container != '')                          AS containers,
		    groupUniqArrayIf(service, service != '')                              AS services,
		    groupUniqArrayIf(environment, environment != '')                      AS environments,
		    groupUniqArrayIf(k8s_namespace, k8s_namespace != '')                  AS namespaces,
		    groupUniqArrayIf(metric_name, metric_name IN @podMetricNames)         AS metric_names
		FROM optikk.metrics_series
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE pod = @pod`
	args := chargs.RangeArgs(tenantID, startMs, endMs)
	args = append(args,
		clickhouse.Named("pod", pod),
		clickhouse.Named("podMetricNames", seriesdefs.Pod.MetricNames()),
	)
	var row PodMetaRow
	return row, dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "containerdetail.QueryPodMeta", &row, query, args...)
}

func (r *Repository) QueryPodRED(ctx context.Context, tenantID int64, pod string, startMs, endMs int64) (PodREDRow, error) {
	query := `
		SELECT
		    ` + spanstats.Requests + `,
		    ` + spanstats.Errors + `,
		    ` + spanstats.DurationSum + `,
		    toFloat32(quantilesTDigestMerge(0.95)(latency_state)[1]) AS p95_latency_ms
		FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND pod = @pod`
	args := chargs.RangeArgs(tenantID, startMs, endMs)
	args = append(args,
		clickhouse.Named("pod", pod),
	)
	var row PodREDRow
	return row, dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "containerdetail.QueryPodRED", &row, query, args...)
}

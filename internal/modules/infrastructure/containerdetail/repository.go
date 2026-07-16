package containerdetail

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/seriesattr"
)

// Caps series rows per request: MaxBucketPoints buckets x label cardinality.
const maxSeriesRows = 10000

const spanDurationMetric = "traces.span.metrics.duration"

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// QuerySeries returns display-grain buckets for one metric group on a pod,
// broken down by the group's label (container/direction/interface/type).
func (r *Repository) QuerySeries(ctx context.Context, tenantID int64, pod string, startMs, endMs int64, def SeriesDef) ([]SeriesPoint, error) {
	valueExpr := "if(sum(m.val_count) = 0, 0, sum(m.val_sum) / sum(m.val_count))"
	if def.Agg == aggRate {
		// Counters: Delta sums directly; Cumulative uses in-bucket increase.
		valueExpr = "sum(if(fps.temporality = 'Delta', m.val_sum, greatest(m.val_max - m.val_min, 0))) / @bucketGrainSec"
	}

	query := `
		WITH fps AS (
		    SELECT fingerprint,
		           any(temporality) AS temporality,
		           ` + def.LabelSQL + ` AS label
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name IN @metricNames
		    WHERE pod = @pod
		    GROUP BY fingerprint, label
		)
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS time_bucket,
		       fps.label  AS series,
		       ` + valueExpr + ` AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN fps ON m.fingerprint = fps.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY time_bucket, series
		ORDER BY time_bucket ASC, series ASC
		LIMIT @maxRows`

	args := chargs.WithMetricNames(chargs.RollupRangeArgs(tenantID, startMs, endMs), def.MetricNames)
	args = timebucket.WithBucketGrainSec(args, startMs, endMs)
	args = append(args,
		clickhouse.Named("pod", pod),
		clickhouse.Named("maxRows", uint64(maxSeriesRows)),
	)
	var rows []SeriesPoint
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "containerdetail.QuerySeries", &rows, query, args...)
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
		clickhouse.Named("podMetricNames", allSeriesMetricNames),
	)
	var row podMetaRow
	return row, dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "containerdetail.QueryPodMeta", &row, query, args...)
}

// QueryPodRED returns range RED aggregates for one pod from span metrics.
func (r *Repository) QueryPodRED(ctx context.Context, tenantID int64, pod string, startMs, endMs int64) (podREDRow, error) {
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           ` + seriesattr.StatusCode + ` AS status_code
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = @spanMetric
		    WHERE pod = @pod
		    GROUP BY fingerprint, status_code
		)
		SELECT
		    sum(m.hist_count)                                                      AS request_count,
		    sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)                AS error_count,
		    sum(m.hist_sum)                                                        AS duration_ms_sum,
		    toFloat32(quantilesPrometheusHistogramMerge(0.95)(m.latency_state)[1]) AS p95_latency_ms
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = @spanMetric`
	args := chargs.RollupRangeArgs(tenantID, startMs, endMs)
	args = append(args,
		clickhouse.Named("pod", pod),
		clickhouse.Named("spanMetric", spanDurationMetric),
	)
	var row podREDRow
	return row, dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "containerdetail.QueryPodRED", &row, query, args...)
}

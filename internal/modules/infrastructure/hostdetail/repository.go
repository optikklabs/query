package hostdetail

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/shared/chargs"
)

// Caps series rows per request: MaxBucketPoints buckets x label cardinality.
const maxSeriesRows = 10000

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// QuerySeries returns display-grain buckets for one metric group on a host,
// broken down by the group's label (state/direction/device/mountpoint).
func (r *Repository) QuerySeries(ctx context.Context, tenantID int64, host string, startMs, endMs int64, def SeriesDef) ([]SeriesPoint, error) {
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
		    WHERE host = @host
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
		clickhouse.Named("host", host),
		clickhouse.Named("maxRows", uint64(maxSeriesRows)),
	)
	var rows []SeriesPoint
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "hostdetail.QuerySeries", &rows, query, args...)
}

// aboutAttrSQL selects the latest non-empty value of one host resource
// attribute retained by ingest in the resource_attributes JSON column.
func aboutAttrSQL(key, alias string) string {
	expr := "coalesce(resource_attributes.`" + key + "`::String, '')"
	return "anyLastIf(" + expr + ", " + expr + " != '') AS " + alias
}

// QueryHostMeta returns identity metadata and which system metrics the host
// reports in range, from the narrow series-metadata table.
func (r *Repository) QueryHostMeta(ctx context.Context, tenantID int64, host string, startMs, endMs int64) (hostMetaRow, error) {
	query := `
		SELECT
		    max(timestamp)                                                        AS last_seen,
		    groupUniqArrayIf(environment, environment != '')                      AS environments,
		    groupUniqArrayIf(k8s_namespace, k8s_namespace != '')                  AS namespaces,
		    groupUniqArrayIf(metric_name, metric_name IN @systemMetricNames)      AS metric_names,
		    ` + aboutAttrSQL("os.type", "os_type") + `,
		    ` + aboutAttrSQL("os.description", "os_description") + `,
		    ` + aboutAttrSQL("host.arch", "host_arch") + `,
		    ` + aboutAttrSQL("host.id", "host_id") + `,
		    ` + aboutAttrSQL("cloud.provider", "cloud_provider") + `,
		    ` + aboutAttrSQL("cloud.platform", "cloud_platform") + `,
		    ` + aboutAttrSQL("cloud.region", "cloud_region") + `,
		    ` + aboutAttrSQL("cloud.availability_zone", "cloud_zone") + `,
		    ` + aboutAttrSQL("k8s.node.name", "k8s_node_name") + `
		FROM optikk.metrics_series
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		WHERE host = @host`
	args := chargs.RangeArgs(tenantID, startMs, endMs)
	args = append(args,
		clickhouse.Named("host", host),
		clickhouse.Named("systemMetricNames", allSeriesMetricNames),
	)
	var row hostMetaRow
	return row, dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "hostdetail.QueryHostMeta", &row, query, args...)
}

var kpiMetricNames = []string{
	infraconsts.MetricSystemCPUUtilization,
	infraconsts.MetricSystemMemoryUtilization,
	infraconsts.MetricSystemFilesystemUtil,
	infraconsts.MetricSystemCPULoadAvg1m,
	infraconsts.MetricSystemCPULoadAvg5m,
	infraconsts.MetricSystemCPULoadAvg15m,
	infraconsts.MetricSystemProcessCount,
}

// QueryKPIs returns range-averaged values per metric/state/mountpoint,
// folded into header KPIs by the service.
func (r *Repository) QueryKPIs(ctx context.Context, tenantID int64, host string, startMs, endMs int64) ([]kpiRow, error) {
	query := `
		WITH fps AS (
		    SELECT fingerprint,
		           metric_name        AS mname,
		           ` + attrState + `      AS mstate,
		           ` + attrMountpoint + ` AS mmount
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name IN @metricNames
		    WHERE host = @host
		    GROUP BY fingerprint, mname, mstate, mmount
		)
		SELECT
		    fps.mname  AS metric_name,
		    fps.mstate AS state,
		    fps.mmount AS mount,
		    if(sum(m.val_count) = 0, 0, sum(m.val_sum) / sum(m.val_count)) AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN fps ON m.fingerprint = fps.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY metric_name, state, mount`
	args := chargs.WithMetricNames(chargs.RollupRangeArgs(tenantID, startMs, endMs), kpiMetricNames)
	args = append(args, clickhouse.Named("host", host))
	var rows []kpiRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "hostdetail.QueryKPIs", &rows, query, args...)
}

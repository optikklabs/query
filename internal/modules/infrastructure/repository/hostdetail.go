package repository

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/models"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesdefs"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
	"github.com/optikklabs/query/internal/shared/chargs"
)

type HostMetaRow struct {
	LastSeen      time.Time `ch:"last_seen"`
	Environments  []string  `ch:"environments"`
	Namespaces    []string  `ch:"namespaces"`
	MetricNames   []string  `ch:"metric_names"`
	OSType        string    `ch:"os_type"`
	OSDescription string    `ch:"os_description"`
	HostArch      string    `ch:"host_arch"`
	HostID        string    `ch:"host_id"`
	CloudProvider string    `ch:"cloud_provider"`
	CloudPlatform string    `ch:"cloud_platform"`
	CloudRegion   string    `ch:"cloud_region"`
	CloudZone     string    `ch:"cloud_zone"`
	K8SNodeName   string    `ch:"k8s_node_name"`
}

type KPIRow struct {
	MetricName string  `ch:"metric_name"`
	State      string  `ch:"state"`
	Mount      string  `ch:"mount"`
	Value      float64 `ch:"value"`
}

func (r *Repository) QueryHostSeries(ctx context.Context, tenantID int64, host string, startMs, endMs int64, def seriesgroup.Def) ([]models.SeriesPoint, error) {
	return r.series.QuerySeries(ctx, tenantID, hostScopeColumn, host, startMs, endMs, def)
}

func aboutAttrSQL(key, alias string) string {
	expr := "resource_attributes['" + key + "']"
	return "argMaxIf(" + expr + ", (timestamp, fingerprint), " + expr + " != '') AS " + alias
}

func (r *Repository) QueryHostMeta(ctx context.Context, tenantID int64, host string, startMs, endMs int64) (HostMetaRow, error) {
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
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE host = @host`
	args := chargs.RangeArgs(tenantID, startMs, endMs)
	args = append(args,
		clickhouse.Named("host", host),
		clickhouse.Named("systemMetricNames", seriesdefs.Host.MetricNames()),
	)
	var row HostMetaRow
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

func (r *Repository) QueryKPIs(ctx context.Context, tenantID int64, host string, startMs, endMs int64) ([]KPIRow, error) {
	query := `
		SELECT
		    metric_name,
		    ` + seriesdefs.AttrState + `      AS state,
		    ` + seriesdefs.AttrMountpoint + ` AS mount,
		    if(sum(val_count) = 0, 0, sum(val_sum) / sum(val_count)) AS value
		FROM ` + timebucket.MetricsRollup(startMs, endMs) + `
		PREWHERE tenant_id     = @tenantID
		     AND metric_name IN @metricNames
		     AND timestamp >= @start AND timestamp < @end
		     AND host = @host
		GROUP BY metric_name, state, mount`
	args := chargs.WithMetricNames(chargs.RangeArgs(tenantID, startMs, endMs), kpiMetricNames)
	args = append(args, clickhouse.Named("host", host))
	var rows []KPIRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "hostdetail.QueryKPIs", &rows, query, args...)
}

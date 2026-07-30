package cloud

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

const (
	MaxProviders  = 8
	MaxCategories = 64
	MaxAccounts   = 50
	MaxResources  = 50
	restartMetric = "k8s.container.restarts"
)

const entityExpr = `if(pod != '', pod,
                        if(k8s_node != '', k8s_node,
                           if(host != '', host, service)))`

// cloud.*/k8s.node.name are resource attributes; reading them from the
// datapoint attributes map returned nothing.
const invSeries = `
	WITH cloud_series AS (
	    SELECT resource_attributes['cloud.provider']   AS provider,
	           resource_attributes['cloud.account.id'] AS account,
	           if(resource_attributes['cloud.region'] != '',
	              resource_attributes['cloud.region'],
	              resource_attributes['aws.region'])   AS region,
	           resource_attributes['cloud.platform']   AS platform,
	           resource_attributes['k8s.node.name']    AS k8s_node,
	           pod, host, service,
	           ` + entityExpr + ` AS entity,
	           max(timestamp)     AS last_seen_ts
	    FROM optikk.metrics_series
	    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
	    WHERE resource_attributes['cloud.provider'] != ''
	    GROUP BY provider, account, region, platform, k8s_node, pod, host, service
	)`

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) QueryProviderInventory(ctx context.Context, tenantID, startMs, endMs int64) ([]InventoryRow, error) {
	query := invSeries + `
		SELECT provider,
		       uniqCombined64If(account, account != '')     AS accounts,
		       uniqCombined64If(region, region != '')       AS regions,
		       uniqCombined64If(k8s_node, k8s_node != '')   AS nodes,
		       uniqCombined64If(pod, pod != '')             AS pods,
		       uniqCombined64If(platform, platform != '')   AS platforms,
		       uniqCombined64(entity)                       AS resources,
		       max(last_seen_ts)                  AS last_seen
		FROM cloud_series
		GROUP BY provider
		ORDER BY resources DESC
		LIMIT @maxProviders`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("maxProviders", uint64(MaxProviders)),
	)
	rows := make([]InventoryRow, 0)
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "cloud.QueryProviderInventory", &rows, query, args...)
}

func (r *Repository) QueryProviderCategories(ctx context.Context, tenantID, startMs, endMs int64) ([]CategoryRow, error) {
	query := invSeries + `
		SELECT provider, platform, uniqCombined64(entity) AS count
		FROM cloud_series
		WHERE platform != ''
		GROUP BY provider, platform
		ORDER BY count DESC
		LIMIT @maxCategories`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("maxCategories", uint64(MaxCategories)),
	)
	rows := make([]CategoryRow, 0)
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "cloud.QueryProviderCategories", &rows, query, args...)
}

func (r *Repository) QueryProviderHealth(ctx context.Context, tenantID, startMs, endMs int64) ([]HealthRow, error) {
	query := `
		SELECT cloud_provider     AS provider,
		       ` + entityExpr + ` AS entity,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND cloud_provider != ''
		GROUP BY provider, entity`
	args := chargs.RollupRangeArgs(tenantID, startMs, endMs)
	rows := make([]HealthRow, 0)
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "cloud.QueryProviderHealth", &rows, query, args...)
}

func (r *Repository) QueryRestarts(ctx context.Context, tenantID, startMs, endMs int64) ([]RestartRow, error) {
	rollupTable := timebucket.MetricsRollup(endMs - startMs)
	query := `
		SELECT provider, toUInt64(sum(latest)) AS restarts
		FROM (
		    SELECT cloud_provider              AS provider,
		           pod,
		           argMax(val_max, timestamp)  AS latest
		    FROM ` + rollupTable + `
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		         AND metric_name = '` + restartMetric + `'
		    WHERE cloud_provider != ''
		    GROUP BY provider, pod
		)
		GROUP BY provider`
	args := chargs.RollupRangeArgs(tenantID, startMs, endMs)
	rows := make([]RestartRow, 0)
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "cloud.QueryRestarts", &rows, query, args...)
}

func (r *Repository) QueryAccountBreakdown(ctx context.Context, tenantID int64, provider string, startMs, endMs int64) ([]AccountRow, error) {
	query := invSeries + `
		SELECT account,
		       uniq(entity)                     AS resources,
		       uniqIf(k8s_node, k8s_node != '') AS nodes,
		       uniqIf(pod, pod != '')           AS pods
		FROM cloud_series
		WHERE provider = @provider AND account != ''
		GROUP BY account
		ORDER BY resources DESC
		LIMIT @maxAccounts`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("provider", provider),
		clickhouse.Named("maxAccounts", uint64(MaxAccounts)),
	)
	rows := make([]AccountRow, 0)
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "cloud.QueryAccountBreakdown", &rows, query, args...)
}

func (r *Repository) QueryPlatformServices(ctx context.Context, tenantID int64, provider string, startMs, endMs int64) ([]CategoryRow, error) {
	query := invSeries + `
		SELECT provider, platform, uniq(entity) AS count
		FROM cloud_series
		WHERE provider = @provider AND platform != ''
		GROUP BY provider, platform
		ORDER BY count DESC
		LIMIT @maxCategories`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("provider", provider),
		clickhouse.Named("maxCategories", uint64(MaxCategories)),
	)
	rows := make([]CategoryRow, 0)
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "cloud.QueryPlatformServices", &rows, query, args...)
}

func (r *Repository) QueryProviderResources(ctx context.Context, tenantID int64, provider string, startMs, endMs int64) ([]ResourceRow, error) {
	query := `
		SELECT ` + entityExpr + `  AS entity,
		       any(service)        AS service_any,
		       any(cloud_region)   AS region,
		       any(cloud_platform) AS platform,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.DurationSum + `
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND cloud_provider = @provider
		GROUP BY entity
		ORDER BY ` + spanstats.ErrorTotal + ` DESC, ` + spanstats.RequestTotal + ` DESC
		LIMIT @maxResources`
	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("provider", provider),
		clickhouse.Named("maxResources", uint64(MaxResources)),
	)
	rows := make([]ResourceRow, 0)
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "cloud.QueryProviderResources", &rows, query, args...)
}

package cloud

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

// Query limits keep every scan bounded (CLAUDE.md §9).
const (
	MaxProviders  = 8
	MaxCategories = 64
	MaxAccounts   = 50
	MaxResources  = 50
	restartMetric = "k8s.container.restarts"
)

// A monitored entity resolves to the first non-empty of pod → node → host →
// service, so the same telemetry row is counted once regardless of signal.
const entityExpr = `if(pod != '', pod,
                        if(k8s_node != '', k8s_node,
                           if(host != '', host, service)))`

// invSeries collapses metrics_series to one row per fingerprint with its cloud
// dimensions, so downstream aggregates count distinct entities not time series.
const invSeries = `
	WITH cloud_series AS (
	    SELECT fingerprint,
	           attributes['cloud.provider']                                            AS provider,
	           attributes['cloud.account.id']                                          AS account,
	           if(attributes['cloud.region'] != '',
	              attributes['cloud.region'], attributes['aws.region'])           AS region,
	           attributes['cloud.platform']                                            AS platform,
	           attributes['k8s.node.name']                                             AS k8s_node,
	           pod, host, service,
	           ` + entityExpr + `                                                           AS entity,
	           max(timestamp)                                                               AS last_seen_ts
	    FROM optikk.metrics_series
	    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
	    WHERE attributes['cloud.provider'] != ''
	    GROUP BY fingerprint, provider, account, region, platform, k8s_node, pod, host, service
	)`

// span_stats carries the cloud dims as real columns, so RED/health reads it
// directly with the shared entity resolution expression.

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// QueryProviderInventory returns one inventory aggregate per cloud provider.
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

// QueryProviderCategories returns per-provider, per-platform entity counts.
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

// QueryProviderHealth returns per-provider, per-entity RED aggregates so the
// service can classify entity health (same source as the nodes module).
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

// QueryRestarts returns the summed latest container-restart count per provider.
// Empty when the collector does not export kubeletstats k8s metrics.
func (r *Repository) QueryRestarts(ctx context.Context, tenantID, startMs, endMs int64) ([]RestartRow, error) {
	rollupTable := timebucket.MetricsRollup(endMs - startMs)
	query := `
		WITH series AS (
		    SELECT fingerprint,
		           attributes['cloud.provider'] AS provider,
		           pod
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = '` + restartMetric + `'
		    WHERE attributes['cloud.provider'] != ''
		    GROUP BY fingerprint, provider, pod
		)
		SELECT provider, toUInt64(sum(latest)) AS restarts
		FROM (
		    SELECT s.provider                     AS provider,
		           s.pod                          AS pod,
		           argMax(m.val_max, m.timestamp) AS latest
		    FROM ` + rollupTable + ` AS m
		    INNER JOIN series AS s ON m.fingerprint = s.fingerprint
		    PREWHERE m.tenant_id = @tenantID AND m.timestamp BETWEEN @start AND @end AND m.metric_name = '` + restartMetric + `'
		    GROUP BY provider, pod
		)
		GROUP BY provider`
	args := chargs.RollupRangeArgs(tenantID, startMs, endMs)
	rows := make([]RestartRow, 0)
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "cloud.QueryRestarts", &rows, query, args...)
}

// QueryAccountBreakdown returns per-account resource counts for one provider.
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

// QueryPlatformServices returns per-platform entity counts for one provider.
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

// QueryProviderResources returns the top entities needing attention for one
// provider, sorted by error volume then request volume.
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

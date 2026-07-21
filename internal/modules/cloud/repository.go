package cloud

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/seriesattr"
)

// Query limits keep every scan bounded (CLAUDE.md §9).
const (
	MaxProviders  = 8
	MaxCategories = 64
	MaxAccounts   = 50
	MaxResources  = 50
	spanMetric    = "traces.span.metrics.duration"
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

// redSeries carries the spanmetrics dimensions + status_code for RED/health.
// The CTE is named `series` because seriesattr.StatusErrorPred references that
// alias (shared with the infrastructure/nodes module).
const redSeries = `
	WITH series AS (
	    SELECT fingerprint,
	           attributes['cloud.provider']                                            AS provider,
	           attributes['cloud.platform']                                            AS platform,
	           if(attributes['cloud.region'] != '',
	              attributes['cloud.region'], attributes['aws.region'])           AS region,
	           attributes['k8s.node.name']                                             AS k8s_node,
	           pod, host, service,
	           ` + entityExpr + `                                                           AS entity,
	           ` + seriesattr.StatusCode + `                                                AS status_code
	    FROM optikk.metrics_series
	    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name = '` + spanMetric + `'
	    WHERE attributes['cloud.provider'] != ''
	    GROUP BY fingerprint, provider, platform, region, k8s_node, pod, host, service, status_code
	)`

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
		       uniqIf(account, account != '')     AS accounts,
		       uniqIf(region, region != '')       AS regions,
		       uniqIf(k8s_node, k8s_node != '')   AS nodes,
		       uniqIf(pod, pod != '')             AS pods,
		       uniqIf(platform, platform != '')   AS platforms,
		       uniq(entity)                       AS resources,
		       max(last_seen_ts)                  AS last_seen
		FROM cloud_series
		GROUP BY provider
		ORDER BY resources DESC
		LIMIT @maxProviders`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("maxProviders", uint64(MaxProviders)),
	)
	var rows []InventoryRow
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "cloud.QueryProviderInventory", &rows, query, args...)
}

// QueryProviderCategories returns per-provider, per-platform entity counts.
func (r *Repository) QueryProviderCategories(ctx context.Context, tenantID, startMs, endMs int64) ([]CategoryRow, error) {
	query := invSeries + `
		SELECT provider, platform, uniq(entity) AS count
		FROM cloud_series
		WHERE platform != ''
		GROUP BY provider, platform
		ORDER BY count DESC
		LIMIT @maxCategories`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("maxCategories", uint64(MaxCategories)),
	)
	var rows []CategoryRow
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "cloud.QueryProviderCategories", &rows, query, args...)
}

// QueryProviderHealth returns per-provider, per-entity RED aggregates so the
// service can classify entity health (same source as the nodes module).
func (r *Repository) QueryProviderHealth(ctx context.Context, tenantID, startMs, endMs int64) ([]HealthRow, error) {
	query := redSeries + `
		SELECT series.provider                                          AS provider,
		       series.entity                                            AS entity,
		       sum(m.hist_count)                                        AS request_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `)  AS error_count
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id = @tenantID
		     AND m.timestamp BETWEEN @start AND @end
		     AND m.metric_name = '` + spanMetric + `'
		GROUP BY provider, entity`
	args := chargs.RollupRangeArgs(tenantID, startMs, endMs)
	var rows []HealthRow
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
	var rows []RestartRow
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
	var rows []AccountRow
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
	var rows []CategoryRow
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "cloud.QueryPlatformServices", &rows, query, args...)
}

// QueryProviderResources returns the top entities needing attention for one
// provider, sorted by error volume then request volume.
func (r *Repository) QueryProviderResources(ctx context.Context, tenantID int64, provider string, startMs, endMs int64) ([]ResourceRow, error) {
	query := redSeries + `
		SELECT series.entity                                           AS entity,
		       any(series.service)                                     AS service,
		       any(series.region)                                      AS region,
		       any(series.platform)                                    AS platform,
		       sum(m.hist_count)                                       AS request_count,
		       sumIf(m.hist_count, ` + seriesattr.StatusErrorPred + `) AS error_count,
		       sum(m.hist_sum)                                         AS duration_ms_sum
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id = @tenantID
		     AND m.timestamp BETWEEN @start AND @end
		     AND m.metric_name = '` + spanMetric + `'
		WHERE series.provider = @provider
		GROUP BY entity
		ORDER BY error_count DESC, request_count DESC
		LIMIT @maxResources`
	args := append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("provider", provider),
		clickhouse.Named("maxResources", uint64(MaxResources)),
	)
	var rows []ResourceRow
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "cloud.QueryProviderResources", &rows, query, args...)
}

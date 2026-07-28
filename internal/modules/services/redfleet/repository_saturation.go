package redfleet

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
)

// saturationCTEs narrows metrics_series to the service's own series plus
// series from hosts the service runs on, before the rollup join.
func saturationCTEs(rangeMs int64) string {
	return `
		WITH service_hosts AS (
		    SELECT DISTINCT host
		    FROM ` + timebucket.SpanStatsRollup(rangeMs) + `
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		      AND service = @serviceName AND host != ''
		),
		fps AS (
		    SELECT fingerprint
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID
		         AND metric_name IN @metricNames
		         AND timestamp BETWEEN @start AND @end
		    WHERE service = @serviceName
		       OR (host != '' AND host IN (SELECT host FROM service_hosts))
		    GROUP BY fingerprint
		)`
}

func saturationArgs(tenantID, startMs, endMs int64, serviceName string, metricNames []string) []any {
	return append(chargs.RollupRangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("metricNames", metricNames),
	)
}

func (r *Repository) GetServiceSaturationAggs(
	ctx context.Context, tenantID int64, startMs, endMs int64, serviceName string, metricNames []string,
) ([]serviceMetricRow, error) {

	query := saturationCTEs(endMs-startMs) + `
		SELECT
		    @serviceName                      AS service,
		    m.metric_name                     AS metric_name,
		    sum(m.val_sum) / sum(m.val_count) AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN fps AS s ON m.fingerprint = s.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY metric_name`

	var rows []serviceMetricRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetServiceSaturationAggs",
		&rows, query, saturationArgs(tenantID, startMs, endMs, serviceName, metricNames)...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) GetServiceSaturationTimeSeries(
	ctx context.Context, tenantID int64, startMs, endMs int64, serviceName string, metricNames []string,
) ([]saturationTimeSeriesRawRow, error) {
	grainSQL := timebucket.DisplayGrainSQL(endMs - startMs)
	query := saturationCTEs(endMs-startMs) + `
		SELECT
		    ` + grainSQL + ` AS bucket_at,
		    sum(m.val_sum) / sum(m.val_count) AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN fps AS s ON m.fingerprint = s.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`

	var rows []saturationTimeSeriesRawRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetServiceSaturationTimeSeries",
		&rows, query, saturationArgs(tenantID, startMs, endMs, serviceName, metricNames)...); err != nil {
		return nil, err
	}
	return rows, nil
}

package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/services/redfleet/models"
	"github.com/optikklabs/query/internal/shared/chargs"
)

// service_hosts stays: the host set is span-derived, so it cannot come from the
// metrics rollup.
func saturationCTEs(startMs, endMs int64) string {
	return `
		WITH service_hosts AS (
		    SELECT DISTINCT host
		    FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + `
		    PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		      AND service = @serviceName AND host != ''
		)`
}

// saturationScope keeps the service's own series and its hosts' series, of
// the utilization series only the ones infraconsts.UsageValueSQL reads.
const saturationScope = `
		WHERE (service = @serviceName OR (host != '' AND host IN service_hosts))
		  AND ` + infraconsts.UsageFilterSQL

func saturationArgs(tenantID, startMs, endMs int64, serviceName string, metricNames []string) []any {
	return append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("serviceName", serviceName),
		clickhouse.Named("metricNames", metricNames),
	)
}

func (r *Repository) GetServiceSaturationAggs(
	ctx context.Context, tenantID int64, startMs, endMs int64, serviceName string, metricNames []string,
) ([]models.ServiceMetricRow, error) {
	query := saturationCTEs(startMs, endMs) + `
		SELECT
		    @serviceName AS service,
		    metric_name,
		    ` + infraconsts.UsageValueSQL + ` AS value
		FROM ` + timebucket.MetricsRollup(startMs, endMs) + `
		PREWHERE tenant_id     = @tenantID
		     AND metric_name IN @metricNames
		     AND timestamp >= @start AND timestamp < @end` + saturationScope + `
		GROUP BY metric_name`

	var rows []models.ServiceMetricRow
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetServiceSaturationAggs",
		&rows, query, saturationArgs(tenantID, startMs, endMs, serviceName, metricNames)...)
	return rows, err
}

func (r *Repository) GetServiceSaturationTimeSeries(
	ctx context.Context, tenantID int64, startMs, endMs int64, serviceName string, metricNames []string,
) ([]models.SaturationPointRow, error) {
	query := saturationCTEs(startMs, endMs) + `
		SELECT
		    ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		    metric_name,
		    ` + infraconsts.UsageValueSQL + ` AS value
		FROM ` + timebucket.MetricsRollup(startMs, endMs) + `
		PREWHERE tenant_id     = @tenantID
		     AND metric_name IN @metricNames
		     AND timestamp >= @start AND timestamp < @end` + saturationScope + `
		GROUP BY bucket_at, metric_name
		ORDER BY bucket_at ASC`

	var rows []models.SaturationPointRow
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "redfleet.GetServiceSaturationTimeSeries",
		&rows, query, saturationArgs(tenantID, startMs, endMs, serviceName, metricNames)...)
	return rows, err
}

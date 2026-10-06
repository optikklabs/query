package repository

import (
	"context"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/shared/chargs"
)

type metricAverageRow struct {
	MetricName string  `ch:"metric_name"`
	Value      float64 `ch:"value"`
}

// QueryMetricAverages returns the fleet-wide mean usage ratio of each named
// metric that has samples in the window, keyed by metric name.
func (r *Repository) QueryMetricAverages(ctx context.Context, tenantID, startMs, endMs int64, metricNames []string) (map[string]float64, error) {
	query := `
		SELECT metric_name,
		       ` + infraconsts.UsageValueSQL + ` AS value
		FROM ` + timebucket.MetricsRollup(startMs, endMs) + `
		PREWHERE tenant_id = @tenantID
		     AND metric_name IN @metricNames
		     AND timestamp >= @start AND timestamp < @end
		WHERE ` + infraconsts.UsageFilterSQL + `
		GROUP BY metric_name`
	args := chargs.WithMetricNames(chargs.RangeArgs(tenantID, startMs, endMs), metricNames)
	var rows []metricAverageRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "infrastructure.QueryMetricAverages", &rows, query, args...); err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(rows))
	for _, row := range rows {
		out[row.MetricName] = row.Value
	}
	return out, nil
}

package repository

import (
	"context"
	"time"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type OpsRaw struct {
	TimeBucket time.Time `ch:"time_bucket"`
	GroupBy    string    `ch:"group_by"`
	OpsPerSec  float64   `ch:"ops_per_sec"`
}

func (r *Repository) GetOpsBySystem(ctx context.Context, tenantID, startMs, endMs int64, f filter.Filters) ([]OpsRaw, error) {
	filterWhere, filterArgs := filter.BuildMetricsClauses(f)

	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS time_bucket,
		       db_system                                        AS group_by,
		       sum(request_count) / @bucketGrainSec             AS ops_per_sec
		FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		WHERE ` + spanstats.DBSpanPred + filterWhere + `
		GROUP BY time_bucket, group_by
		ORDER BY time_bucket, group_by`

	args := append(chargs.RangeArgs(tenantID, startMs, endMs), filterArgs...)
	args = timebucket.WithBucketGrainSec(args, startMs, endMs)
	var rows []OpsRaw
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "volume.GetOpsBySystem", &rows, query, args...)
}

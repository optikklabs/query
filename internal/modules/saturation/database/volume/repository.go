package volume

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetOpsBySystem(ctx context.Context, tenantID, startMs, endMs int64, f filter.Filters) ([]opsRawDTO, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	filterWhere, filterArgs := filter.BuildMetricsClauses(f)

	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS time_bucket,
		       db_system                                        AS group_by,
		       sum(request_count) / @bucketGrainSec             AS ops_per_sec
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		WHERE ` + spanstats.DBSpanPred + filterWhere + `
		GROUP BY time_bucket, group_by
		ORDER BY time_bucket, group_by`

	args := append(chargs.RangeArgs(tenantID, startMs, endMs), filterArgs...)
	args = timebucket.WithBucketGrainSec(args, startMs, endMs)
	var rows []opsRawDTO
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "volume.GetOpsBySystem", &rows, query, args...)
}

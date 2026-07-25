package latency

import (
	"context"
	"time"

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

type latencyRawDTO struct {
	BucketAt time.Time `ch:"bucket_at"`
	GroupBy  string    `ch:"group_by"`
	QS       []float32 `ch:"qs"`
	P50Ms    float32
	P95Ms    float32
	P99Ms    float32
}

func (r *Repository) GetLatencyBySystem(ctx context.Context, tenantID, startMs, endMs int64, f filter.Filters) ([]latencyRawDTO, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	filterWhere, filterArgs := filter.BuildMetricsClauses(f)
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       db_system                                        AS group_by,
		       arrayMap(x -> toFloat32(x), quantilesTDigestMerge(0.5, 0.95, 0.99)(latency_state)) AS qs
		FROM ` + timebucket.SpanStatsRollup(endMs-startMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		WHERE ` + spanstats.DBSpanPred + filterWhere + `
		GROUP BY bucket_at, group_by
		ORDER BY bucket_at, group_by`

	args := append(chargs.RangeArgs(tenantID, startMs, endMs), filterArgs...)
	var rows []latencyRawDTO
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "latency.GetLatencyBySystem", &rows, query, args...); err != nil {
		return nil, err
	}
	for i := range rows {
		if len(rows[i].QS) >= 3 {
			rows[i].P50Ms = rows[i].QS[0]
			rows[i].P95Ms = rows[i].QS[1]
			rows[i].P99Ms = rows[i].QS[2]
		}
	}
	return rows, nil
}

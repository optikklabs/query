package latency

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
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

func (r *Repository) GetLatencyBySystem(ctx context.Context, teamID, startMs, endMs int64, f filter.Filters) ([]latencyRawDTO, error) {
	return r.latencySeriesByGroup(ctx, teamID, startMs, endMs, f, filter.AttrDBSystem, "latency.GetLatencyBySystem")
}

func (r *Repository) latencySeriesByGroup(ctx context.Context, teamID, startMs, endMs int64, f filter.Filters, attr, traceLabel string) ([]latencyRawDTO, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	groupCol := filter.MetricsGroupColumn(attr)
	if groupCol == "" {
		return nil, nil
	}
	filterWhere, filterArgs := filter.BuildMetricsClauses(f)
	query := `
		WITH series AS (
		    SELECT fingerprint, ` + groupCol + ` AS group_by
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name = 'traces.span.metrics.duration'
		    WHERE attributes.` + "`db.system`" + `::String != ''` + filterWhere + `
		    GROUP BY fingerprint, group_by
		)
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       series.group_by                                  AS group_by,
		       arrayMap(x -> toFloat32(x), quantilesPrometheusHistogramMerge(0.5, 0.95, 0.99)(m.latency_state)) AS qs
		FROM ` + timebucket.MetricsHistRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.timestamp   BETWEEN @start AND @end
		     AND m.metric_name = 'traces.span.metrics.duration'
		GROUP BY bucket_at, group_by
		ORDER BY bucket_at, group_by`

	args := append(filter.SpanArgs(teamID, startMs, endMs), filterArgs...)
	var rows []latencyRawDTO
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, traceLabel, &rows, query, args...); err != nil {
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

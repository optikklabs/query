package seriesgroup

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
)

type Point struct {
	TimeBucket time.Time `json:"timeBucket" ch:"time_bucket"`
	Series     string    `json:"series"      ch:"series"`
	Value      float64   `json:"value"       ch:"value"`
}

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) QuerySeries(
	ctx context.Context, tenantID int64, scopeCol, scopeVal string, startMs, endMs int64, def Def,
) ([]Point, error) {
	valueExpr := "if(sum(val_count) = 0, 0, sum(val_sum) / sum(val_count))"
	if def.Agg == Rate {

		valueExpr = "sum(if(temporality = 'Delta', val_sum, greatest(val_max - val_min, 0))) / @bucketGrainSec"
	}

	query := `
		SELECT ` + timebucket.DisplayGrainSQLForRange(startMs, endMs) + ` AS time_bucket,
		       ` + def.LabelSQL + ` AS series,
		       ` + valueExpr + ` AS value
		FROM ` + timebucket.MetricsRollup(startMs, endMs) + `
		PREWHERE tenant_id     = @tenantID
		     AND metric_name IN @metricNames
		     AND timestamp >= @start AND timestamp < @end
		     AND ` + scopeCol + ` = @scopeVal
		GROUP BY time_bucket, series
		ORDER BY time_bucket ASC, series ASC`

	args := chargs.WithMetricNames(chargs.RangeArgs(tenantID, startMs, endMs), def.MetricNames)
	args = timebucket.WithBucketGrainSec(args, startMs, endMs)
	args = append(args, clickhouse.Named("scopeVal", scopeVal))
	var rows []Point
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "seriesgroup.QuerySeries."+scopeCol,
		&rows, query, args...)
}

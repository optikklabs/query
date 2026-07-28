package seriesgroup

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
)

const maxSeriesRows = 10000

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
	valueExpr := "if(sum(m.val_count) = 0, 0, sum(m.val_sum) / sum(m.val_count))"
	if def.Agg == Rate {

		valueExpr = "sum(if(fps.temporality = 'Delta', m.val_sum, greatest(m.val_max - m.val_min, 0))) / @bucketGrainSec"
	}

	query := `
		WITH fps AS (
		    SELECT fingerprint,
		           any(temporality) AS temporality,
		           ` + def.LabelSQL + ` AS label
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name IN @metricNames
		    WHERE ` + scopeCol + ` = @scopeVal
		    GROUP BY fingerprint, label
		)
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS time_bucket,
		       fps.label  AS series,
		       ` + valueExpr + ` AS value
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN fps ON m.fingerprint = fps.fingerprint
		PREWHERE m.tenant_id     = @tenantID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY time_bucket, series
		ORDER BY time_bucket ASC, series ASC
		LIMIT @maxRows`

	args := chargs.WithMetricNames(chargs.RollupRangeArgs(tenantID, startMs, endMs), def.MetricNames)
	args = timebucket.WithBucketGrainSec(args, startMs, endMs)
	args = append(args,
		clickhouse.Named("scopeVal", scopeVal),
		clickhouse.Named("maxRows", uint64(maxSeriesRows)),
	)
	var rows []Point
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "seriesgroup.QuerySeries."+scopeCol,
		&rows, query, args...)
}

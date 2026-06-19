package producer

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/saturation/kafka/filter"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// No producer instrumentation is emitted; produce rate is derived from the
// growth of the broker-scraped partition log-end offset (kafka.partition.
// current_offset). Per-minute growth (val_max - val_min) feeds the counter-rate
// fold. This approximates: inter-minute boundary gaps between scrapes are not
// counted, so it can undercount under sustained load.
var produceOffsetMetrics = []string{"kafka.partition.current_offset"}

// QueryPublishRateByTopic returns publish rate grouped by topic.
func (r *Repository) QueryPublishRateByTopic(ctx context.Context, teamID int64, startMs, endMs int64) ([]TopicCounterRow, error) {
	const query = `
		SELECT
		    timestamp,
		    messaging_destination          AS topic,
		    greatest(val_max - val_min, 0) AS value
		FROM optikk.metrics_1m -- pinned to 1m: Go-side rate folds assume per-minute rows
		PREWHERE team_id     = @teamID
		     AND ts_bucket   BETWEEN @bucketStart AND @bucketEnd
		     AND metric_name IN @metricNames
		     AND timestamp   BETWEEN @start AND @end
		WHERE messaging_destination != ''
		  AND lower(messaging_system) = 'kafka'
		ORDER BY timestamp`
	args := filter.WithMetricNames(filter.MetricArgs(teamID, startMs, endMs), produceOffsetMetrics)
	var rows []TopicCounterRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryPublishRateByTopic", &rows, query, args...)
}

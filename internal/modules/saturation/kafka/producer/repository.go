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
// current_offset). Per-bucket growth (val_max - val_min) feeds the counter-rate
// fold, which divides the summed deltas by the display-grain seconds — so the
// source-bucket grain (5m) does not affect the result.
var produceOffsetMetrics = []string{"kafka.partition.current_offset"}

// QueryPublishRateByTopic returns publish rate grouped by topic.
func (r *Repository) QueryPublishRateByTopic(ctx context.Context, teamID int64, startMs, endMs int64) ([]TopicCounterRow, error) {
	const query = `
		WITH series AS (
		    SELECT fingerprint, any(` + filter.AttrTopic + `) AS topic
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name IN @metricNames
		    WHERE ` + filter.AttrTopic + ` != '' AND lower(` + filter.AttrSystem + `) = 'kafka'
		    GROUP BY fingerprint
		)
		SELECT
		    toStartOfFiveMinutes(m.timestamp)  AS timestamp,
		    series.topic                       AS topic,
		    greatest(max(m.value) - min(m.value), 0) AS value
		FROM optikk.metrics AS m -- rate fold divides by display-grain seconds (grain-independent)
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY timestamp, topic
		ORDER BY timestamp`
	args := filter.WithMetricNames(filter.MetricArgs(teamID, startMs, endMs), produceOffsetMetrics)
	var rows []TopicCounterRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryPublishRateByTopic", &rows, query, args...)
}

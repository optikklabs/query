package consumer

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

// Counter rate panels.

const counterSeriesByTopicQuery = `
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

// No consumer-throughput counter is emitted; consume rate is derived from the
// growth of the broker-scraped committed offset (kafka.consumer_group.offset),
// summed across groups/partitions per topic. Same per-bucket-delta approximation
// as the produce side.
var consumeOffsetMetrics = []string{"kafka.consumer_group.offset"}

func (r *Repository) QueryConsumeRateByTopic(ctx context.Context, teamID int64, startMs, endMs int64) ([]TopicCounterRow, error) {
	args := filter.WithMetricNames(filter.MetricArgs(teamID, startMs, endMs), consumeOffsetMetrics)
	var rows []TopicCounterRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryConsumeRateByTopic", &rows, counterSeriesByTopicQuery, args...)
}

// Lag panels.

func (r *Repository) QueryConsumerLagByGroupTopic(ctx context.Context, teamID int64, startMs, endMs int64) ([]GroupTopicGaugeRow, error) {
	const query = `
		WITH series AS (
		    SELECT fingerprint,
		           any(` + filter.AttrConsumerGroup + `) AS consumer_group,
		           any(` + filter.AttrTopic + `)         AS topic
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name IN @metricNames
		    WHERE ` + filter.AttrConsumerGroup + ` != ''
		    GROUP BY fingerprint
		)
		SELECT
		    toStartOfFiveMinutes(m.timestamp)    AS timestamp,
		    series.consumer_group                AS consumer_group,
		    series.topic                         AS topic,
		    avg(m.value)                         AS value
		FROM optikk.metrics AS m -- rate fold divides by display-grain seconds (grain-independent)
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY timestamp, consumer_group, topic
		ORDER BY timestamp`
	args := filter.WithMetricNames(filter.MetricArgs(teamID, startMs, endMs), filter.ConsumerLagMetrics)
	var rows []GroupTopicGaugeRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryConsumerLagByGroupTopic", &rows, query, args...)
}

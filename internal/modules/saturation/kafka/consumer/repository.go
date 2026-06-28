package consumer

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/saturation/kafka/filter"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// Counter rate panels.

// counterSeriesByTopicQuery buckets at the display grain so the per-bucket sum
// matches the seconds the fold divides by (a fixed 5m bucket would mismatch
// finer windows).
func counterSeriesByTopicQuery(windowMs int64) string {
	return `
		WITH series AS (
		    SELECT fingerprint, ` + filter.AttrTopic + ` AS topic
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name IN @metricNames
		    WHERE ` + filter.AttrTopic + ` != '' AND lower(` + filter.AttrSystem + `) = 'kafka'
		    GROUP BY fingerprint, topic
		)
		SELECT
		    ` + timebucket.DisplayGrainSQL(windowMs) + ` AS timestamp,
		    series.topic                       AS topic,
		    sum(m.value)                       AS value
		FROM optikk.metrics AS m -- delta counter: per-bucket sum; fold divides by display-grain seconds
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY timestamp, topic
		ORDER BY timestamp`
}

var consumeCounterMetrics = []string{"kafka.consumer.records_consumed_total"}

func (r *Repository) QueryConsumeRateByTopic(ctx context.Context, teamID int64, startMs, endMs int64) ([]TopicCounterRow, error) {
	args := filter.WithMetricNames(filter.MetricArgs(teamID, startMs, endMs), consumeCounterMetrics)
	var rows []TopicCounterRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryConsumeRateByTopic", &rows, counterSeriesByTopicQuery(endMs-startMs), args...)
}

func consumerLagByGroupTopicQuery(windowMs int64) string {
	return `
		WITH series AS (
		    SELECT fingerprint,
		           ` + filter.AttrConsumerGroup + ` AS consumer_group,
		           ` + filter.AttrTopic + `         AS topic
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name IN @metricNames
		    WHERE ` + filter.AttrConsumerGroup + ` != ''
		    GROUP BY fingerprint, consumer_group, topic
		)
		SELECT
		    ` + timebucket.DisplayGrainSQL(windowMs) + ` AS timestamp,
		    series.consumer_group                AS consumer_group,
		    series.topic                         AS topic,
		    avg(m.value)                         AS value
		FROM optikk.metrics AS m -- gauge avg per display-grain bucket
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.team_id     = @teamID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp   BETWEEN @start AND @end
		GROUP BY timestamp, consumer_group, topic
		ORDER BY timestamp`
}

func (r *Repository) QueryConsumerLagByGroupTopic(ctx context.Context, teamID int64, startMs, endMs int64) ([]GroupTopicGaugeRow, error) {
	args := filter.WithMetricNames(filter.MetricArgs(teamID, startMs, endMs), filter.ConsumerLagMetrics)
	var rows []GroupTopicGaugeRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryConsumerLagByGroupTopic", &rows, consumerLagByGroupTopicQuery(endMs-startMs), args...)
}

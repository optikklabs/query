package producer

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

// Produce rate is derived from the client-side producer counter
// kafka.producer.record_send_total (per topic). The OTel agent exports it with
// delta temporality, so each point is the count since the last export; the
// per-bucket sum feeds the counter-rate fold, which divides by display-grain
// seconds.
var produceCounterMetrics = []string{"kafka.producer.record_send_total"}

func publishRateByTopicQuery(windowMs int64) string {
	return `
		WITH series AS (
		    SELECT fingerprint, any(` + filter.AttrTopic + `) AS topic
		    FROM optikk.metrics_series
		    PREWHERE team_id = @teamID AND timestamp BETWEEN @start AND @end AND metric_name IN @metricNames
		    WHERE ` + filter.AttrTopic + ` != '' AND lower(` + filter.AttrSystem + `) = 'kafka'
		    GROUP BY fingerprint
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

func (r *Repository) QueryPublishRateByTopic(ctx context.Context, teamID int64, startMs, endMs int64) ([]TopicCounterRow, error) {
	args := filter.WithMetricNames(filter.MetricArgs(teamID, startMs, endMs), produceCounterMetrics)
	var rows []TopicCounterRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryPublishRateByTopic", &rows, publishRateByTopicQuery(endMs-startMs), args...)
}

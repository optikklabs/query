package explorer

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

// buildFilterArgs constructs the optional dimension filter (applied in the
// metrics_series CTE) and the bind args for queries.
func buildFilterArgs(tenantID, startMs, endMs int64, metricNames []string, filterCol, filterVal string) (string, []any) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	args := filter.WithMetricNames(filter.MetricArgs(tenantID, startMs, endMs), metricNames)
	var extraWhere string
	if filterVal != "" {
		if filterCol == "topic" {
			extraWhere = "AND " + filter.AttrTopic + " = @filterVal"
		} else if filterCol == "consumer_group" {
			extraWhere = "AND " + filter.AttrConsumerGroup + " = @filterVal"
		}
		args = append(args, clickhouse.Named("filterVal", filterVal))
	}
	return extraWhere, args
}

func seriesCTE(needTopic, needGroup bool, baseWhere, extraWhere string) string {
	sel := "fingerprint"
	grp := "fingerprint"
	if needTopic {
		sel += ", " + filter.AttrTopic + " AS topic"
		grp += ", topic"
	}
	if needGroup {
		sel += ", " + filter.AttrConsumerGroup + " AS consumer_group"
		grp += ", consumer_group"
	}
	return `WITH series AS (
		    SELECT ` + sel + `
		    FROM optikk.metrics_series
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND metric_name IN @metricNames
		    WHERE ` + baseWhere + ` ` + extraWhere + `
		    GROUP BY ` + grp + `
		)
		`
}

var topicThroughputMetrics = []string{
	"kafka.consumer.bytes_consumed_rate",
	"kafka.consumer.bytes_consumed_total",
	"kafka.consumer.records_consumed_rate",
	"kafka.consumer.records_consumed_total",
}

func (r *Repository) QueryTopicThroughput(ctx context.Context, tenantID, startMs, endMs int64, topic string) ([]TopicThroughputRow, error) {
	extraWhere, args := buildFilterArgs(tenantID, startMs, endMs, topicThroughputMetrics, "topic", topic)
	query := seriesCTE(true, false, filter.AttrTopic+" != ''", extraWhere) + `
		SELECT
		    series.topic AS topic,
		    avg(if(metric_name = 'kafka.consumer.bytes_consumed_rate',
		           ifNotFinite(val_sum / val_count, 0), NULL))    AS bytes_per_sec,
		    max(if(metric_name = 'kafka.consumer.bytes_consumed_total',
		           ifNotFinite(val_sum / val_count, 0), NULL))    AS bytes_total,
		    avg(if(metric_name = 'kafka.consumer.records_consumed_rate',
		           ifNotFinite(val_sum / val_count, 0), NULL))    AS records_per_sec,
		    max(if(metric_name = 'kafka.consumer.records_consumed_total',
		           ifNotFinite(val_sum / val_count, 0), NULL))    AS records_total
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id   = @tenantID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp BETWEEN @start AND @end
		GROUP BY topic
		ORDER BY bytes_per_sec DESC, topic ASC`
	rows := make([]TopicThroughputRow, 0)
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryTopicThroughput", &rows, query, args...)
}

var groupPartitionMetrics = []string{"kafka.consumer_group.lag", "kafka.consumer_group.members"}

func (r *Repository) QueryGroupPartitions(ctx context.Context, tenantID, startMs, endMs int64, group string) ([]GroupPartitionsRow, error) {
	extraWhere, args := buildFilterArgs(tenantID, startMs, endMs, groupPartitionMetrics, "consumer_group", group)
	query := seriesCTE(true, true, filter.AttrConsumerGroup+" != ''", extraWhere) + `
		SELECT
		    series.consumer_group AS consumer_group,
		    toFloat64(countDistinctIf(m.fingerprint, metric_name = 'kafka.consumer_group.lag')) AS assigned_partitions,
		    countDistinctIf(series.topic, series.topic != '' AND metric_name = 'kafka.consumer_group.lag') AS topic_count,
		    ifNotFinite(argMaxIf(val_max, timestamp, metric_name = 'kafka.consumer_group.members'), 0) AS members
		FROM ` + timebucket.MetricsRollup(endMs-startMs) + ` AS m
		INNER JOIN series ON m.fingerprint = series.fingerprint
		PREWHERE m.tenant_id   = @tenantID
		     AND m.metric_name IN @metricNames
		     AND m.timestamp BETWEEN @start AND @end
		GROUP BY consumer_group
		ORDER BY assigned_partitions DESC, consumer_group ASC`
	rows := make([]GroupPartitionsRow, 0)
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryGroupPartitions", &rows, query, args...)
}

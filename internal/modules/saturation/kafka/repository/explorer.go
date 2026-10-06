package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/saturation/kafka/filter"
	"github.com/optikklabs/query/internal/modules/saturation/kafka/models"
	"github.com/optikklabs/query/internal/shared/chargs"
)

// buildFilterArgs binds the window and metric names, plus an optional
// equality filter on attrExpr when value is set.
func buildFilterArgs(tenantID, startMs, endMs int64, metricNames []string, attrExpr, value string) (string, []any) {
	args := chargs.WithMetricNames(chargs.RangeArgs(tenantID, startMs, endMs), metricNames)
	if value == "" {
		return "", args
	}
	return "AND " + attrExpr + " = @filterVal", append(args, clickhouse.Named("filterVal", value))
}

var topicThroughputMetrics = []string{
	"kafka.consumer.bytes_consumed_rate",
	"kafka.consumer.bytes_consumed_total",
	"kafka.consumer.records_consumed_rate",
	"kafka.consumer.records_consumed_total",
}

func (r *Repository) QueryTopicThroughput(ctx context.Context, tenantID, startMs, endMs int64, topic string) ([]models.TopicThroughputRow, error) {
	extraWhere, args := buildFilterArgs(tenantID, startMs, endMs, topicThroughputMetrics, filter.AttrTopic, topic)
	query := `
		SELECT
		    ` + filter.AttrTopic + ` AS topic,
		    avg(if(metric_name = 'kafka.consumer.bytes_consumed_rate',
		           ifNotFinite(val_sum / val_count, 0), NULL))    AS bytes_per_sec,
		    max(if(metric_name = 'kafka.consumer.bytes_consumed_total',
		           ifNotFinite(val_sum / val_count, 0), NULL))    AS bytes_total,
		    avg(if(metric_name = 'kafka.consumer.records_consumed_rate',
		           ifNotFinite(val_sum / val_count, 0), NULL))    AS records_per_sec,
		    max(if(metric_name = 'kafka.consumer.records_consumed_total',
		           ifNotFinite(val_sum / val_count, 0), NULL))    AS records_total
		FROM ` + timebucket.MetricsRollup(startMs, endMs) + `
		PREWHERE tenant_id   = @tenantID
		     AND metric_name IN @metricNames
		     AND timestamp >= @start AND timestamp < @end
		WHERE ` + filter.AttrTopic + ` != '' ` + extraWhere + `
		GROUP BY topic
		ORDER BY bytes_per_sec DESC, topic ASC
		LIMIT 200`
	rows := make([]models.TopicThroughputRow, 0)
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryTopicThroughput", &rows, query, args...)
	return rows, err
}

var groupPartitionMetrics = []string{"kafka.consumer_group.lag", "kafka.consumer_group.members"}

func (r *Repository) QueryGroupPartitions(ctx context.Context, tenantID, startMs, endMs int64, group string) ([]models.GroupPartitionsRow, error) {
	extraWhere, args := buildFilterArgs(tenantID, startMs, endMs, groupPartitionMetrics, filter.AttrConsumerGroup, group)
	query := `
		SELECT
		    ` + filter.AttrConsumerGroup + ` AS consumer_group,
		    toFloat64(countDistinctIf(fingerprint, metric_name = 'kafka.consumer_group.lag')) AS assigned_partitions,
		    countDistinctIf(` + filter.AttrTopic + `, ` + filter.AttrTopic + ` != '' AND metric_name = 'kafka.consumer_group.lag') AS topic_count,
		    ifNotFinite(argMaxIf(val_max, timestamp, metric_name = 'kafka.consumer_group.members'), 0) AS members
		FROM ` + timebucket.MetricsRollup(startMs, endMs) + `
		PREWHERE tenant_id   = @tenantID
		     AND metric_name IN @metricNames
		     AND timestamp >= @start AND timestamp < @end
		WHERE ` + filter.AttrConsumerGroup + ` != '' ` + extraWhere + `
		GROUP BY consumer_group
		ORDER BY assigned_partitions DESC, consumer_group ASC
		LIMIT 200`
	rows := make([]models.GroupPartitionsRow, 0)
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryGroupPartitions", &rows, query, args...)
	return rows, err
}

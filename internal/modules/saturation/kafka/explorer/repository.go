package explorer

import (
	"context"
	"fmt"

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

// buildFilterArgs constructs filtering clauses and arguments for queries.
func buildFilterArgs(teamID, startMs, endMs int64, metricNames []string, filterCol, filterVal string) (string, []any) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	args := filter.WithMetricNames(filter.MetricArgs(teamID, startMs, endMs), metricNames)
	var extraWhere string
	if filterVal != "" {
		if filterCol == "topic" {
			extraWhere = "AND messaging_destination = @filterVal"
		} else if filterCol == "consumer_group" {
			extraWhere = "AND messaging_consumer_group = @filterVal"
		}
		args = append(args, clickhouse.Named("filterVal", filterVal))
	}
	return extraWhere, args
}

var topicThroughputMetrics = []string{
	"kafka.consumer.bytes_consumed_rate",
	"kafka.consumer.bytes_consumed_total",
	"kafka.consumer.records_consumed_rate",
	"kafka.consumer.records_consumed_total",
}

// QueryTopicThroughput returns consumption rates and totals per topic.
func (r *Repository) QueryTopicThroughput(ctx context.Context, teamID, startMs, endMs int64, topic string) ([]TopicThroughputRow, error) {
	extraWhere, args := buildFilterArgs(teamID, startMs, endMs, topicThroughputMetrics, "topic", topic)
	query := fmt.Sprintf(`
		SELECT
		    messaging_destination AS topic,
		    avg(if(metric_name = 'kafka.consumer.bytes_consumed_rate',
		           ifNotFinite(val_sum / val_count, 0), NULL))    AS bytes_per_sec,
		    max(if(metric_name = 'kafka.consumer.bytes_consumed_total',
		           ifNotFinite(val_sum / val_count, 0), NULL))    AS bytes_total,
		    avg(if(metric_name = 'kafka.consumer.records_consumed_rate',
		           ifNotFinite(val_sum / val_count, 0), NULL))    AS records_per_sec,
		    max(if(metric_name = 'kafka.consumer.records_consumed_total',
		           ifNotFinite(val_sum / val_count, 0), NULL))    AS records_total
		FROM `+timebucket.MetricsRollup(endMs-startMs)+`
		PREWHERE team_id   = @teamID
		     AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		     AND metric_name IN @metricNames
		     AND timestamp BETWEEN @start AND @end
		WHERE messaging_destination != '' %s
		GROUP BY topic
		ORDER BY bytes_per_sec DESC, topic ASC`, extraWhere)
	rows := make([]TopicThroughputRow, 0)
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryTopicThroughput", &rows, query, args...)
}

// Lag is sourced from the kafkametrics receiver (kafka.consumer_group.lag),
// which carries real per (group, topic, partition) lag; lead remains the JMX
// client gauge (no kafkametrics equivalent is emitted).
var topicLagMetrics = []string{
	"kafka.consumer_group.lag",
	"kafka.consumer.records_lead",
}

// QueryTopicLag returns max lag and lead per topic.
func (r *Repository) QueryTopicLag(ctx context.Context, teamID, startMs, endMs int64, topic string) ([]TopicLagRow, error) {
	extraWhere, args := buildFilterArgs(teamID, startMs, endMs, topicLagMetrics, "topic", topic)
	query := fmt.Sprintf(`
		SELECT
		    messaging_destination AS topic,
		    max(if(metric_name = 'kafka.consumer_group.lag',
		           ifNotFinite(val_sum / val_count, 0), NULL))  AS lag,
		    max(if(metric_name = 'kafka.consumer.records_lead',
		           ifNotFinite(val_sum / val_count, 0), NULL))  AS lead
		FROM `+timebucket.MetricsRollup(endMs-startMs)+`
		PREWHERE team_id   = @teamID
		     AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		     AND metric_name IN @metricNames
		     AND timestamp BETWEEN @start AND @end
		WHERE messaging_destination != '' %s
		GROUP BY topic
		ORDER BY lag DESC, topic ASC`, extraWhere)
	rows := make([]TopicLagRow, 0)
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryTopicLag", &rows, query, args...)
}

// QueryTopicConsumers returns count of consumer groups per topic.
func (r *Repository) QueryTopicConsumers(ctx context.Context, teamID, startMs, endMs int64, topic string) ([]TopicConsumersRow, error) {
	extraWhere, args := buildFilterArgs(teamID, startMs, endMs, topicLagMetrics, "topic", topic)
	query := fmt.Sprintf(`
		SELECT
		    messaging_destination AS topic,
		    count(DISTINCT messaging_consumer_group) AS consumer_group_count
		FROM `+timebucket.MetricsRollup(endMs-startMs)+`
		PREWHERE team_id   = @teamID
		     AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		     AND metric_name IN @metricNames
		     AND timestamp BETWEEN @start AND @end
		WHERE messaging_destination != '' %s
		GROUP BY topic
		ORDER BY topic ASC`, extraWhere)
	rows := make([]TopicConsumersRow, 0)
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryTopicConsumers", &rows, query, args...)
}

// Partition count and retained-message backlog come from the kafkametrics
// receiver: per-partition offsets (current - oldest) and the topic partition gauge.
var topicBacklogMetrics = []string{
	"kafka.partition.current_offset",
	"kafka.partition.oldest_offset",
	"kafka.topic.partitions",
}

// QueryTopicBacklog returns partition count and retained-message backlog per topic.
func (r *Repository) QueryTopicBacklog(ctx context.Context, teamID, startMs, endMs int64, topic string) ([]TopicBacklogRow, error) {
	extraWhere, args := buildFilterArgs(teamID, startMs, endMs, topicBacklogMetrics, "topic", topic)
	query := fmt.Sprintf(`
		SELECT topic, max(parts) AS partition_count, sum(backlog) AS backlog
		FROM (
		    SELECT
		        messaging_destination AS topic,
		        fingerprint,
		        argMaxIf(val_max, timestamp, metric_name = 'kafka.topic.partitions') AS parts,
		        greatest(
		            argMaxIf(val_max, timestamp, metric_name = 'kafka.partition.current_offset')
		          - argMaxIf(val_max, timestamp, metric_name = 'kafka.partition.oldest_offset'), 0) AS backlog
		    FROM `+timebucket.MetricsRollup(endMs-startMs)+`
		    PREWHERE team_id   = @teamID
		         AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		         AND metric_name IN @metricNames
		         AND timestamp BETWEEN @start AND @end
		    WHERE messaging_destination != '' %s
		    GROUP BY topic, fingerprint
		)
		GROUP BY topic
		ORDER BY backlog DESC, topic ASC`, extraWhere)
	rows := make([]TopicBacklogRow, 0)
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryTopicBacklog", &rows, query, args...)
}

// Group identity is carried by the kafkametrics-receiver metrics
// (kafka.consumer_group.*), not the JMX kafka.consumer.* client metrics which
// are keyed by client-id only. consumer_group.lag has one series per
// (group, topic, partition), so distinct series == partitions the group reads.
// Partition / topic identity comes from kafka.consumer_group.lag (one series per
// group/topic/partition); members from kafka.consumer_group.members.
var groupPartitionMetrics = []string{"kafka.consumer_group.lag", "kafka.consumer_group.members"}

// QueryGroupPartitions returns partitions, topics, and members per consumer group.
func (r *Repository) QueryGroupPartitions(ctx context.Context, teamID, startMs, endMs int64, group string) ([]GroupPartitionsRow, error) {
	extraWhere, args := buildFilterArgs(teamID, startMs, endMs, groupPartitionMetrics, "consumer_group", group)
	query := fmt.Sprintf(`
		SELECT
		    messaging_consumer_group AS consumer_group,
		    toFloat64(countDistinctIf(fingerprint, metric_name = 'kafka.consumer_group.lag')) AS assigned_partitions,
		    countDistinctIf(messaging_destination, messaging_destination != '' AND metric_name = 'kafka.consumer_group.lag') AS topic_count,
		    ifNotFinite(argMaxIf(val_max, timestamp, metric_name = 'kafka.consumer_group.members'), 0) AS members
		FROM `+timebucket.MetricsRollup(endMs-startMs)+`
		PREWHERE team_id   = @teamID
		     AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		     AND metric_name IN @metricNames
		     AND timestamp BETWEEN @start AND @end
		WHERE messaging_consumer_group != '' %s
		GROUP BY consumer_group
		ORDER BY assigned_partitions DESC, consumer_group ASC`, extraWhere)
	rows := make([]GroupPartitionsRow, 0)
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryGroupPartitions", &rows, query, args...)
}

var groupCommitMetrics = []string{
	"kafka.consumer.commit_rate",
	"kafka.consumer.commit_latency_avg",
	"kafka.consumer.commit_latency_max",
}

// QueryGroupCommits returns commit rate and latencies per consumer group.
func (r *Repository) QueryGroupCommits(ctx context.Context, teamID, startMs, endMs int64, group string) ([]GroupCommitsRow, error) {
	extraWhere, args := buildFilterArgs(teamID, startMs, endMs, groupCommitMetrics, "consumer_group", group)
	query := fmt.Sprintf(`
		SELECT
		    messaging_consumer_group AS consumer_group,
		    avg(if(metric_name = 'kafka.consumer.commit_rate',
		           ifNotFinite(val_sum / val_count, 0), NULL))        AS commit_rate,
		    ifNotFinite(avg(if(metric_name = 'kafka.consumer.commit_latency_avg',
		           ifNotFinite(val_sum / val_count, 0), NULL)), 0)    AS commit_latency_avg_ms,
		    max(if(metric_name = 'kafka.consumer.commit_latency_max',
		           ifNotFinite(val_sum / val_count, 0), NULL))        AS commit_latency_max_ms
		FROM `+timebucket.MetricsRollup(endMs-startMs)+`
		PREWHERE team_id   = @teamID
		     AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		     AND metric_name IN @metricNames
		     AND timestamp BETWEEN @start AND @end
		WHERE messaging_consumer_group != '' %s
		GROUP BY consumer_group
		ORDER BY consumer_group ASC`, extraWhere)
	rows := make([]GroupCommitsRow, 0)
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryGroupCommits", &rows, query, args...)
}

var groupFetchMetrics = []string{
	"kafka.consumer.fetch_rate",
	"kafka.consumer.fetch_latency_avg",
	"kafka.consumer.fetch_latency_max",
}

// QueryGroupFetches returns fetch rate and latencies per consumer group.
func (r *Repository) QueryGroupFetches(ctx context.Context, teamID, startMs, endMs int64, group string) ([]GroupFetchesRow, error) {
	extraWhere, args := buildFilterArgs(teamID, startMs, endMs, groupFetchMetrics, "consumer_group", group)
	query := fmt.Sprintf(`
		SELECT
		    messaging_consumer_group AS consumer_group,
		    avg(if(metric_name = 'kafka.consumer.fetch_rate',
		           ifNotFinite(val_sum / val_count, 0), NULL))        AS fetch_rate,
		    ifNotFinite(avg(if(metric_name = 'kafka.consumer.fetch_latency_avg',
		           ifNotFinite(val_sum / val_count, 0), NULL)), 0)    AS fetch_latency_avg_ms,
		    max(if(metric_name = 'kafka.consumer.fetch_latency_max',
		           ifNotFinite(val_sum / val_count, 0), NULL))        AS fetch_latency_max_ms
		FROM `+timebucket.MetricsRollup(endMs-startMs)+`
		PREWHERE team_id   = @teamID
		     AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		     AND metric_name IN @metricNames
		     AND timestamp BETWEEN @start AND @end
		WHERE messaging_consumer_group != '' %s
		GROUP BY consumer_group
		ORDER BY consumer_group ASC`, extraWhere)
	rows := make([]GroupFetchesRow, 0)
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryGroupFetches", &rows, query, args...)
}

// QueryClusterHealth returns broker-level health from the kafkametrics receiver
// (kafka.brokers / controller / partition replicas). Gauges are read at their
// latest value in the window; under-replicated counts partitions whose live
// replica set exceeds the in-sync set.
func (r *Repository) QueryClusterHealth(ctx context.Context, teamID, startMs, endMs int64) (ClusterHealthRow, error) {
	startMs, endMs = timebucket.SnapRangeForRollup(startMs, endMs)
	tbl := timebucket.MetricsRollup(endMs - startMs)
	args := filter.MetricArgs(teamID, startMs, endMs)
	query := fmt.Sprintf(`
		SELECT
		    (SELECT ifNull(argMax(val_max, timestamp), 0) FROM %[1]s
		       PREWHERE team_id = @teamID AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		            AND metric_name = 'kafka.brokers' AND timestamp BETWEEN @start AND @end
		    ) AS broker_count,
		    (SELECT ifNull(argMax(val_max, timestamp), 0) FROM %[1]s
		       PREWHERE team_id = @teamID AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		            AND metric_name = 'kafka.controller.active.count' AND timestamp BETWEEN @start AND @end
		    ) AS active_controllers,
		    (SELECT countIf(replicas > insync) FROM (
		        SELECT fingerprint,
		               argMaxIf(val_max, timestamp, metric_name = 'kafka.partition.replicas')         AS replicas,
		               argMaxIf(val_max, timestamp, metric_name = 'kafka.partition.replicas_in_sync') AS insync
		        FROM %[1]s
		        PREWHERE team_id = @teamID AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		             AND metric_name IN ('kafka.partition.replicas', 'kafka.partition.replicas_in_sync')
		             AND timestamp BETWEEN @start AND @end
		        GROUP BY fingerprint
		    )) AS under_replicated_partitions`, tbl)
	rows := make([]ClusterHealthRow, 0, 1)
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryClusterHealth", &rows, query, args...); err != nil {
		return ClusterHealthRow{}, err
	}
	if len(rows) == 0 {
		return ClusterHealthRow{}, nil
	}
	return rows[0], nil
}

var groupHealthMetrics = []string{
	"kafka.consumer.heartbeat_rate",
	"kafka.consumer.failed_rebalance_rate_per_hour",
	"kafka.consumer.poll_idle_ratio_avg",
	"kafka.consumer.last_poll_seconds_ago",
	"kafka.consumer.connection_count",
}

// QueryGroupHealth returns health metrics per consumer group.
func (r *Repository) QueryGroupHealth(ctx context.Context, teamID, startMs, endMs int64, group string) ([]GroupHealthRow, error) {
	extraWhere, args := buildFilterArgs(teamID, startMs, endMs, groupHealthMetrics, "consumer_group", group)
	query := fmt.Sprintf(`
		SELECT
		    messaging_consumer_group AS consumer_group,
		    avg(if(metric_name = 'kafka.consumer.heartbeat_rate',
		           ifNotFinite(val_sum / val_count, 0), NULL))                    AS heartbeat_rate,
		    avg(if(metric_name = 'kafka.consumer.failed_rebalance_rate_per_hour',
		           ifNotFinite(val_sum / val_count, 0), NULL))                    AS failed_rebalance_per_hour,
		    ifNotFinite(avg(if(metric_name = 'kafka.consumer.poll_idle_ratio_avg',
		           ifNotFinite(val_sum / val_count, 0), NULL)), 0)                AS poll_idle_ratio,
		    max(if(metric_name = 'kafka.consumer.last_poll_seconds_ago',
		           ifNotFinite(val_sum / val_count, 0), NULL))                    AS last_poll_seconds_ago,
		    max(if(metric_name = 'kafka.consumer.connection_count',
		           ifNotFinite(val_sum / val_count, 0), NULL))                    AS connection_count
		FROM `+timebucket.MetricsRollup(endMs-startMs)+`
		PREWHERE team_id   = @teamID
		     AND ts_bucket BETWEEN @bucketStart AND @bucketEnd
		     AND metric_name IN @metricNames
		     AND timestamp BETWEEN @start AND @end
		WHERE messaging_consumer_group != '' %s
		GROUP BY consumer_group
		ORDER BY consumer_group ASC`, extraWhere)
	rows := make([]GroupHealthRow, 0)
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "kafka.QueryGroupHealth", &rows, query, args...)
}

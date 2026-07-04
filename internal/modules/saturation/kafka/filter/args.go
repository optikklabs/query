// Package filter holds SQL and OTel helpers shared by saturation/kafka.
package filter

import (
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const MessagingSystemKafka = "kafka"

const MaxTopQueues = 50

func MetricArgs(tenantID int64, startMs, endMs int64) []any {
	return []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
	}
}

func WithMetricNames(args []any, metricNames []string) []any {
	return append(args, clickhouse.Named("metricNames", metricNames))
}

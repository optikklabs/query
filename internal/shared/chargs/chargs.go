package chargs

import (
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/infra/timebucket"
)

func RangeArgs(tenantID, startMs, endMs int64) []any {
	return []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
	}
}

func RollupRangeArgs(tenantID, startMs, endMs int64) []any {
	if timebucket.UseHourRollup(endMs - startMs) {
		startMs = timebucket.FloorMsToHour(startMs)
	}
	return RangeArgs(tenantID, startMs, endMs)
}

func WithMetricNames(args []any, names []string) []any {
	return append(args, clickhouse.Named("metricNames", names))
}

package database

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/config"
)

const (
	chConnMaxLifetime = 30 * time.Minute
	chDialTimeout     = 5 * time.Second
)

// OpenClickHouseConn opens the shared native-protocol pool. Pool sizing is
// config-driven (clickhouse.max_open_conns / max_idle_conns) so operators
// can tune per deployment without a rebuild.
func OpenClickHouseConn(dsn string, maxOpenConns, maxIdleConns int) (clickhouse.Conn, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: parse DSN: %w", err)
	}

	opts.Compression = &clickhouse.Compression{Method: clickhouse.CompressionLZ4}
	opts.MaxOpenConns = maxOpenConns
	opts.MaxIdleConns = maxIdleConns
	opts.ConnMaxLifetime = chConnMaxLifetime
	opts.DialTimeout = chDialTimeout
	opts.ConnOpenStrategy = clickhouse.ConnOpenRoundRobin

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := conn.Ping(pingCtx); err != nil {
		return nil, err
	}

	return conn, nil
}

// Per-budget ClickHouse settings applied via clickhouse.Context.
// Keeps dashboard queries prioritized over background explorer scans.
var dashboardSettings = clickhouse.Settings{
	"max_execution_time":              10,
	"max_rows_to_read":                300_000_000,
	"max_memory_usage":                2 * 1024 * 1024 * 1024,
	"max_result_rows":                 100_000,
	"result_overflow_mode":            "throw",
	"read_overflow_mode":              "throw",
	"optimize_read_in_order":          1,
	"use_query_cache":                 1,
	"query_cache_ttl":                 60,
	"query_cache_share_between_users": 0,
	"use_query_condition_cache":       1,
	"max_threads":                     4,
	"priority":                        1,
}

var overviewSettings = clickhouse.Settings{
	"max_execution_time":              30,
	"max_rows_to_read":                500_000_000,
	"max_memory_usage":                4 * 1024 * 1024 * 1024,
	"max_result_rows":                 100_000,
	"result_overflow_mode":            "throw",
	"read_overflow_mode":              "throw",
	"optimize_read_in_order":          1,
	"use_query_cache":                 1,
	"query_cache_ttl":                 60,
	"query_cache_share_between_users": 0,
	"use_query_condition_cache":       1,
	"max_threads":                     8,
	"priority":                        5,
}

var explorerSettings = clickhouse.Settings{
	"max_execution_time":              60,
	"max_rows_to_read":                1_000_000_000,
	"max_memory_usage":                4 * 1024 * 1024 * 1024,
	"max_result_rows":                 100_000,
	"result_overflow_mode":            "throw",
	"read_overflow_mode":              "throw",
	"optimize_read_in_order":          1,
	"use_query_cache":                 1,
	"query_cache_ttl":                 60,
	"query_cache_share_between_users": 0,
	"use_query_condition_cache":       1,
	"max_threads":                     16,
	"priority":                        10,
}

func InitQueryBudgets(budgets config.QueryBudgetsConfig) {
	dashboardSettings["max_execution_time"] = budgets.Dashboard.MaxExecutionTime
	dashboardSettings["max_rows_to_read"] = budgets.Dashboard.MaxRowsToRead
	dashboardSettings["max_memory_usage"] = budgets.Dashboard.MaxMemoryUsage
	dashboardSettings["max_result_rows"] = budgets.Dashboard.MaxResultRows
	dashboardSettings["max_threads"] = budgets.Dashboard.MaxThreads
	dashboardSettings["priority"] = budgets.Dashboard.Priority

	overviewSettings["max_execution_time"] = budgets.Overview.MaxExecutionTime
	overviewSettings["max_rows_to_read"] = budgets.Overview.MaxRowsToRead
	overviewSettings["max_memory_usage"] = budgets.Overview.MaxMemoryUsage
	overviewSettings["max_result_rows"] = budgets.Overview.MaxResultRows
	overviewSettings["max_threads"] = budgets.Overview.MaxThreads
	overviewSettings["priority"] = budgets.Overview.Priority

	explorerSettings["max_execution_time"] = budgets.Explorer.MaxExecutionTime
	explorerSettings["max_rows_to_read"] = budgets.Explorer.MaxRowsToRead
	explorerSettings["max_memory_usage"] = budgets.Explorer.MaxMemoryUsage
	explorerSettings["max_result_rows"] = budgets.Explorer.MaxResultRows
	explorerSettings["max_threads"] = budgets.Explorer.MaxThreads
	explorerSettings["priority"] = budgets.Explorer.Priority
}

func DashboardCtx(ctx context.Context) context.Context {
	return clickhouse.Context(ctx, clickhouse.WithSettings(dashboardSettings))
}

func OverviewCtx(ctx context.Context) context.Context {
	return clickhouse.Context(ctx, clickhouse.WithSettings(overviewSettings))
}

func ExplorerCtx(ctx context.Context) context.Context {
	return clickhouse.Context(ctx, clickhouse.WithSettings(explorerSettings))
}

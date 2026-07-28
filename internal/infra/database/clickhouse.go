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

// Fallback budgets mirror the config defaults so contexts stay safe even if
// InitQueryBudgets was never called (e.g. in tests).
var (
	dashboardSettings = budgetSettings(config.QueryBudget{
		MaxExecutionTime: 10, MaxRowsToRead: 300_000_000, MaxMemoryUsage: 2 * 1024 * 1024 * 1024,
		MaxResultRows: 100_000, MaxThreads: 4, Priority: 1,
	})
	overviewSettings = budgetSettings(config.QueryBudget{
		MaxExecutionTime: 30, MaxRowsToRead: 500_000_000, MaxMemoryUsage: 4 * 1024 * 1024 * 1024,
		MaxResultRows: 100_000, MaxThreads: 4, Priority: 5,
	})
	explorerSettings = budgetSettings(config.QueryBudget{
		MaxExecutionTime: 60, MaxRowsToRead: 1_000_000_000, MaxMemoryUsage: 4 * 1024 * 1024 * 1024,
		MaxResultRows: 100_000, MaxThreads: 4, Priority: 10,
	})
)

// budgetSettings builds an immutable settings map for one query class;
// the maps are shared by every query context and must never be mutated.
func budgetSettings(b config.QueryBudget) clickhouse.Settings {
	return clickhouse.Settings{
		"max_execution_time":              b.MaxExecutionTime,
		"max_rows_to_read":                b.MaxRowsToRead,
		"max_memory_usage":                b.MaxMemoryUsage,
		"max_result_rows":                 b.MaxResultRows,
		"result_overflow_mode":            "throw",
		"read_overflow_mode":              "throw",
		"optimize_read_in_order":          1,
		"use_query_cache":                 1,
		"query_cache_ttl":                 60,
		"query_cache_share_between_users": 0,
		"use_query_condition_cache":       1,
		"max_threads":                     b.MaxThreads,
		"priority":                        b.Priority,
	}
}

// InitQueryBudgets replaces the settings once at startup, before any
// connection serves queries.
func InitQueryBudgets(budgets config.QueryBudgetsConfig) {
	dashboardSettings = budgetSettings(budgets.Dashboard)
	overviewSettings = budgetSettings(budgets.Overview)
	explorerSettings = budgetSettings(budgets.Explorer)
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

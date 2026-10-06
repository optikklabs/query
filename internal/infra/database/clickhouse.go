package database

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/optikklabs/query/internal/config"
)

const (
	chConnMaxLifetime = 30 * time.Minute
	chDialTimeout     = 5 * time.Second
)

// OpenClickHouseConn connects with opts plus the shared pool, compression
// and dial settings, and verifies the connection.
func OpenClickHouseConn(opts *clickhouse.Options) (clickhouse.Conn, error) {
	opts.Compression = &clickhouse.Compression{Method: clickhouse.CompressionLZ4}
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
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

// Query budgets per query class, set once at startup by InitQueryBudgets
// before any connection serves queries.
var dashboardSettings, overviewSettings, explorerSettings clickhouse.Settings

// budgetSettings builds an immutable settings map for one query class;
// the maps are shared by every query context and must never be mutated.
func budgetSettings(b config.QueryBudget) clickhouse.Settings {
	return clickhouse.Settings{
		"max_execution_time":        b.MaxExecutionTime,
		"max_rows_to_read":          b.MaxRowsToRead,
		"max_memory_usage":          b.MaxMemoryUsage,
		"max_result_rows":           b.MaxResultRows,
		"result_overflow_mode":      "throw",
		"read_overflow_mode":        "throw",
		"optimize_read_in_order":    1,
		"use_query_cache":           0,
		"use_query_condition_cache": 1,
		"max_threads":               b.MaxThreads,
		"priority":                  b.Priority,
		// Bucket functions such as toStartOfDay use UTC whatever the server's
		// timezone, matching the UTC instants the API returns.
		"session_timezone": "UTC",
	}
}

// InitQueryBudgets sets the per-class query budgets.
func InitQueryBudgets(budgets config.QueryBudgetsConfig) {
	dashboardSettings = budgetSettings(budgets.Dashboard)
	overviewSettings = budgetSettings(budgets.Overview)
	explorerSettings = budgetSettings(budgets.Explorer)
}

// WithSettings replaces rather than merges; SelectCH needs the base map.
type budgetKey struct{}

// budgetCtx applies a query class's settings and decodes DateTime columns as
// UTC, so API timestamps never carry the server's local offset.
func budgetCtx(ctx context.Context, settings clickhouse.Settings) context.Context {
	return context.WithValue(
		clickhouse.Context(ctx, clickhouse.WithSettings(settings), clickhouse.WithUserLocation(time.UTC)),
		budgetKey{}, settings,
	)
}

func DashboardCtx(ctx context.Context) context.Context {
	return budgetCtx(ctx, dashboardSettings)
}

func OverviewCtx(ctx context.Context) context.Context {
	return budgetCtx(ctx, overviewSettings)
}

func ExplorerCtx(ctx context.Context) context.Context {
	return budgetCtx(ctx, explorerSettings)
}

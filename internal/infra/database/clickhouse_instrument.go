package database

import (
	"context"
	"maps"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// withOpComment tags the query with op so system.query_log rows attribute
// back to the caller. Every query runs under a budget context.
func withOpComment(ctx context.Context, op string) context.Context {
	base, ok := ctx.Value(budgetKey{}).(clickhouse.Settings)
	if !ok {
		panic("database: ClickHouse query " + op + " has no budget context")
	}
	settings := make(clickhouse.Settings, len(base)+1)
	maps.Copy(settings, base)
	settings["log_comment"] = op
	return clickhouse.Context(ctx, clickhouse.WithSettings(settings))
}

func SelectCH(ctx context.Context, conn clickhouse.Conn, op string, dest any, query string, args ...any) error {
	ctx = withOpComment(ctx, op)
	done := instrument(ctx, "clickhouse", op)
	err := wrapBudgetExceeded(conn.Select(ctx, dest, query, args...))
	done(err)
	return err
}

// QueryRowCH scans one row into dest and returns sql.ErrNoRows when the
// query matched none.
func QueryRowCH(ctx context.Context, conn clickhouse.Conn, op string, dest any, query string, args ...any) error {
	ctx = withOpComment(ctx, op)
	done := instrument(ctx, "clickhouse", op)
	err := wrapBudgetExceeded(conn.QueryRow(ctx, query, args...).ScanStruct(dest))
	done(err)
	return err
}

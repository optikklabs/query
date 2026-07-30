package database

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"maps"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/infra/metrics"
)

// Tags the query with op so system.query_log rows attribute back to the caller.
func withOpComment(ctx context.Context, op string) context.Context {
	base, ok := ctx.Value(budgetKey{}).(clickhouse.Settings)
	if !ok {
		return ctx
	}
	settings := make(clickhouse.Settings, len(base)+1)
	maps.Copy(settings, base)
	settings["log_comment"] = op
	return clickhouse.Context(ctx, clickhouse.WithSettings(settings))
}

func SelectCH(ctx context.Context, conn clickhouse.Conn, op string, dest any, query string, args ...any) error {
	ctx = withOpComment(ctx, op)
	done := startCHOp(ctx)
	start := time.Now()
	err := wrapBudgetExceeded(conn.Select(ctx, dest, query, args...))
	done(err, start, op)
	return err
}

func QueryRowCH(ctx context.Context, conn clickhouse.Conn, op string, dest any, query string, args ...any) error {
	ctx = withOpComment(ctx, op)
	done := startCHOp(ctx)
	start := time.Now()
	err := conn.QueryRow(ctx, query, args...).ScanStruct(dest)

	if err != nil && isNoRows(err) {
		done(nil, start, op)
		return nil
	}
	err = wrapBudgetExceeded(err)
	done(err, start, op)
	return err
}

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func startCHOp(ctx context.Context) func(error, time.Time, string) {
	return func(err error, start time.Time, op string) {
		dur := time.Since(start).Seconds()
		metrics.DBQueryDuration.WithLabelValues("clickhouse", op).Observe(dur)
		metrics.DBQueriesTotal.WithLabelValues("clickhouse", op, resultLabel(err)).Inc()
		if err != nil {
			slog.ErrorContext(ctx, "clickhouse query failed",
				slog.String("op", op),
				slog.Float64("duration_s", dur),
				slog.Any("error", err),
			)
		}
	}
}

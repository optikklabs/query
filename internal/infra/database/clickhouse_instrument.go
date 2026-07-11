package database

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/infra/metrics"
)

func SelectCH(ctx context.Context, conn clickhouse.Conn, op string, dest any, query string, args ...any) error {
	done := startCHOp(ctx)
	start := time.Now()
	err := conn.Select(ctx, dest, query, args...)
	done(err, start, op)
	return err
}

func QueryRowCH(ctx context.Context, conn clickhouse.Conn, op string, dest any, query string, args ...any) error {
	done := startCHOp(ctx)
	start := time.Now()
	err := conn.QueryRow(ctx, query, args...).ScanStruct(dest)
	if err != nil && isNoRows(err) {
		done(nil, start, op)
		return nil
	}
	done(err, start, op)
	return err
}

// isNoRows reports whether err means "query matched zero rows".
// clickhouse-go's QueryRow path returns exactly sql.ErrNoRows for an empty
// result; matching anything looser (e.g. "EOF" substrings) masks dropped
// connections as empty results.
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

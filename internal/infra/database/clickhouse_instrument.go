package database

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/infra/metrics"
	types "github.com/optikklabs/query/internal/shared/contracts"
)

func SelectCH(ctx context.Context, conn clickhouse.Conn, op string, dest any, query string, args ...any) error {
	key := coalesceKey(types.TenantFrom(ctx).TenantID, query, args)
	return coalesce(ctx, key, op, dest, func(runCtx context.Context, out any) error {
		done := startCHOp(runCtx)
		start := time.Now()
		err := conn.Select(runCtx, out, query, args...)
		done(err, start, op)
		return err
	})
}

func QueryRowCH(ctx context.Context, conn clickhouse.Conn, op string, dest any, query string, args ...any) error {
	key := coalesceKey(types.TenantFrom(ctx).TenantID, query, args)
	return coalesce(ctx, key, op, dest, func(runCtx context.Context, out any) error {
		done := startCHOp(runCtx)
		start := time.Now()
		err := conn.QueryRow(runCtx, query, args...).ScanStruct(out)

		if err != nil && isNoRows(err) {
			done(nil, start, op)
			return nil
		}
		done(err, start, op)
		return err
	})
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

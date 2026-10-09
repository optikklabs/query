package database

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/optikklabs/query/internal/infra/metrics"
)

// instrument starts timing a query against system ("mysql", "clickhouse");
// the returned func records its metrics and logs a failure.
func instrument(ctx context.Context, system, op string) func(error) {
	start := time.Now()
	return func(err error) {
		dur := time.Since(start).Seconds()
		metrics.DBQueryDuration.WithLabelValues(system, op).Observe(dur)
		metrics.DBQueriesTotal.WithLabelValues(system, op, resultLabel(err)).Inc()
		if isFailure(err) {
			slog.ErrorContext(ctx, system+" query failed",
				slog.String("op", op),
				slog.Float64("duration_s", dur),
				slog.Any("error", err),
			)
		}
	}
}

// isFailure reports whether err is a real query failure. A lookup that
// matched no row and a unique-key conflict are expected outcomes that
// services map to 404 and 409.
func isFailure(err error) bool {
	return err != nil && !errors.Is(err, sql.ErrNoRows) && !IsDuplicateEntry(err)
}

func resultLabel(err error) string {
	if isFailure(err) {
		return "err"
	}
	return "ok"
}

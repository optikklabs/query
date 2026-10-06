package database

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/optikklabs/query/internal/infra/metrics"
)

func GetSQL(ctx context.Context, db *sqlx.DB, op string, dest any, query string, args ...any) error {
	done := startSQLOp(ctx)
	start := time.Now()
	err := db.GetContext(ctx, dest, query, args...)
	done(err, start, op)
	return err
}

func SelectSQL(ctx context.Context, db *sqlx.DB, op string, dest any, query string, args ...any) error {
	done := startSQLOp(ctx)
	start := time.Now()
	err := db.SelectContext(ctx, dest, query, args...)
	done(err, start, op)
	return err
}

func ExecSQL(ctx context.Context, db *sqlx.DB, op, query string, args ...any) (sql.Result, error) {
	done := startSQLOp(ctx)
	start := time.Now()
	res, err := db.ExecContext(ctx, query, args...)
	done(err, start, op)
	return res, err
}

// ExecMatched runs a write that must hit a row and returns sql.ErrNoRows when
// none matched. Connections report matched rather than changed rows, so an
// UPDATE that rewrites identical values still counts as a match.
func ExecMatched(ctx context.Context, db *sqlx.DB, op, query string, args ...any) error {
	res, err := ExecSQL(ctx, db, op, query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// NoRowsAs maps sql.ErrNoRows to notFound and passes every other error,
// including nil, through unchanged.
func NoRowsAs(err, notFound error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return notFound
	}
	return err
}

func startSQLOp(ctx context.Context) func(error, time.Time, string) {
	return func(err error, start time.Time, op string) {
		dur := time.Since(start).Seconds()
		metrics.DBQueryDuration.WithLabelValues("mysql", op).Observe(dur)
		metrics.DBQueriesTotal.WithLabelValues("mysql", op, resultLabel(err)).Inc()
		if isFailure(err) {
			slog.ErrorContext(ctx, "mysql query failed",
				slog.String("op", op),
				slog.Float64("duration_s", dur),
				slog.Any("error", err),
			)
		}
	}
}

package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func GetSQL(ctx context.Context, db *sqlx.DB, op string, dest any, query string, args ...any) error {
	done := instrument(ctx, "mysql", op)
	err := db.GetContext(ctx, dest, query, args...)
	done(err)
	return err
}

func SelectSQL(ctx context.Context, db *sqlx.DB, op string, dest any, query string, args ...any) error {
	done := instrument(ctx, "mysql", op)
	err := db.SelectContext(ctx, dest, query, args...)
	done(err)
	return err
}

func ExecSQL(ctx context.Context, db *sqlx.DB, op, query string, args ...any) (sql.Result, error) {
	done := instrument(ctx, "mysql", op)
	res, err := db.ExecContext(ctx, query, args...)
	done(err)
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

// IsDuplicateEntry reports whether err is a MySQL unique-key violation.
func IsDuplicateEntry(err error) bool {
	const duplicateEntry = 1062
	me, ok := errors.AsType[*mysql.MySQLError](err)
	return ok && me.Number == duplicateEntry
}

// DuplicateAs maps a unique-key violation to conflict, passing other errors
// through.
func DuplicateAs(err, conflict error) error {
	if IsDuplicateEntry(err) {
		return conflict
	}
	return err
}

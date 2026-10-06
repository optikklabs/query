package database

import (
	"database/sql"
	"errors"
)

// isFailure reports whether err is a real query failure; a lookup that
// matched no row is an expected outcome, not a failure.
func isFailure(err error) bool {
	return err != nil && !errors.Is(err, sql.ErrNoRows)
}

func resultLabel(err error) string {
	if isFailure(err) {
		return "err"
	}
	return "ok"
}

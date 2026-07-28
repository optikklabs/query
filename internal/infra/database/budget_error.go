package database

import (
	"errors"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

// ClickHouse exception codes raised when a query exceeds the budgets set
// in this package (max_execution_time, max_rows_to_read/read_overflow_mode,
// max_result_rows/result_overflow_mode). Values verified against ch-go
// proto/error_enum.go (ErrTooManyRows, ErrTimeoutExceeded,
// ErrTooManyRowsOrBytes).
const (
	chCodeTooManyRows        = 158
	chCodeTimeoutExceeded    = 159
	chCodeTooManyRowsOrBytes = 396
)

// wrapBudgetExceeded tags budget-violation exceptions with the shared
// sentinel so handlers can surface them as a typed 4xx instead of a 500.
func wrapBudgetExceeded(err error) error {
	if err == nil {
		return nil
	}
	var ex *clickhouse.Exception
	if !errors.As(err, &ex) {
		return err
	}
	switch ex.Code {
	case chCodeTooManyRows, chCodeTimeoutExceeded, chCodeTooManyRowsOrBytes:
		return fmt.Errorf("%w: %w", errorcode.ErrQueryBudgetExceeded, err)
	}
	return err
}

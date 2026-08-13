package errorcode

import "errors"

const (
	BadRequest          = "BAD_REQUEST"
	Validation          = "VALIDATION_ERROR"
	Unauthorized        = "UNAUTHORIZED"
	Forbidden           = "FORBIDDEN"
	NotFound            = "NOT_FOUND"
	Conflict            = "CONFLICT"
	RateLimited         = "RATE_LIMITED"
	TrialExpired        = "TRIAL_EXPIRED"
	QueryBudgetExceeded = "QUERY_BUDGET_EXCEEDED"
)

// ErrQueryBudgetExceeded marks queries rejected by per-query execution
// budgets (row/time limits); the HTTP layer maps it to a typed 422.
var ErrQueryBudgetExceeded = errors.New("query budget exceeded")

// ValidationError marks client-fixable input errors; HTTP maps it to 400.
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return e.Msg }

// NotFoundError marks missing resources; HTTP maps it to 404.
type NotFoundError struct{ Msg string }

func (e NotFoundError) Error() string { return e.Msg }

// ConflictError marks resource-state conflicts; HTTP maps it to 409.
type ConflictError struct{ Msg string }

func (e ConflictError) Error() string { return e.Msg }

// UnauthorizedError marks failed authentication; HTTP maps it to 401.
type UnauthorizedError struct{ Msg string }

func (e UnauthorizedError) Error() string { return e.Msg }

// TrialExpiredError marks suspended tenants; HTTP maps it to 402.
type TrialExpiredError struct{ Msg string }

func (e TrialExpiredError) Error() string { return e.Msg }

const (
	Internal    = "INTERNAL_ERROR"
	QueryFailed = "QUERY_FAILED"
	Unavailable = "SERVICE_UNAVAILABLE"
)

const (
	NoData = "NO_DATA"
)

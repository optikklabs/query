package errorcode

const (
	BadRequest   = "BAD_REQUEST"
	Validation   = "VALIDATION_ERROR"
	Unauthorized = "UNAUTHORIZED"
	Forbidden    = "FORBIDDEN"
	NotFound     = "NOT_FOUND"
	Conflict     = "CONFLICT"
	RateLimited  = "RATE_LIMITED"
	TrialExpired = "TRIAL_EXPIRED"
)

const (
	Internal    = "INTERNAL_ERROR"
	QueryFailed = "QUERY_FAILED"
	Unavailable = "SERVICE_UNAVAILABLE"
)

const (
	NoData = "NO_DATA"
)

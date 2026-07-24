package errorcode

// Request & validation errors (4xx)
const (
	BadRequest      = "BAD_REQUEST"
	Validation      = "VALIDATION_ERROR"
	Unauthorized    = "UNAUTHORIZED"
	Forbidden       = "FORBIDDEN"
	NotFound        = "NOT_FOUND"
	Conflict        = "CONFLICT"
	PayloadTooLarge = "PAYLOAD_TOO_LARGE"
	RateLimited     = "RATE_LIMITED"
	TrialExpired    = "TRIAL_EXPIRED"
)

const (
	Internal        = "INTERNAL_ERROR"
	QueryFailed     = "QUERY_FAILED"
	QueryTimeout    = "QUERY_TIMEOUT"
	ConnectionError = "CONNECTION_ERROR"
	Unavailable     = "SERVICE_UNAVAILABLE"
	CircuitOpen     = "CIRCUIT_OPEN"
)

const (
	NoData      = "NO_DATA"
	PartialData = "PARTIAL_DATA"
)

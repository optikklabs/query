package shared

import (
	"errors"
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

// UnauthorizedError marks failed authentication; HTTP maps it to 401.
type UnauthorizedError struct{ Msg string }

func (e UnauthorizedError) Error() string { return e.Msg }

// TrialExpiredError marks suspended tenants; HTTP maps it to 402.
type TrialExpiredError struct{ Msg string }

func (e TrialExpiredError) Error() string { return e.Msg }

// RespondServiceError maps the auth-only error kinds (401/402) and defers
// validation/not-found/conflict/internal to the shared responder.
func RespondServiceError(w http.ResponseWriter, r *http.Request, err error, failMsg string) {
	var (
		ua UnauthorizedError
		te TrialExpiredError
	)
	switch {
	case errors.As(err, &ua):
		httputil.RespondErrorWithCause(w, r, http.StatusUnauthorized, errorcode.Unauthorized, ua.Msg, nil)
	case errors.As(err, &te):
		httputil.RespondErrorWithCause(w, r, http.StatusPaymentRequired, errorcode.TrialExpired, te.Msg, nil)
	default:
		httputil.RespondServiceError(w, r, err, failMsg)
	}
}

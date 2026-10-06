package httputil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	types "github.com/optikklabs/query/internal/shared/contracts"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

const APIV1Base = "/api/v1"

// maxBodyBytes caps every JSON request body.
const maxBodyBytes = 1 << 20

func Tenant(r *http.Request) types.TenantContext {
	return types.TenantFrom(r.Context())
}

func URLParamLower(r *http.Request, key string) string {
	return strings.ToLower(chi.URLParam(r, key))
}

func ClientIP(r *http.Request) string {
	if ip := middleware.GetClientIP(r.Context()); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// WriteJSON encodes v before committing the status, so a value that cannot
// be encoded (a NaN, say) becomes a 500 failure envelope rather than an empty
// body under the original status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		slog.Error("failed to encode json response", slog.Any("error", err))
		status = http.StatusInternalServerError
		body, _ = json.Marshal(types.Failure(errorcode.Internal, "failed to encode response", "", w.Header().Get("X-Request-Id")))
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(append(body, '\n')); err != nil {
		slog.Error("failed to write json response", slog.Any("error", err))
	}
}

// BindJSON decodes the request body into v and answers 400 when the body is
// not exactly one JSON value matching v.
func BindJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := decodeJSON(w, r, v); err != nil {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "invalid request body: "+err.Error(), nil)
		return false
	}
	return true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(v)
	if err == nil {
		// A second value (or trailing garbage) means the body was not one value.
		if err = decoder.Decode(&struct{}{}); errors.Is(err, io.EOF) {
			return nil
		}
		if err == nil {
			return errors.New("body must contain a single JSON value")
		}
	}
	if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
		return fmt.Errorf("body exceeds %d bytes", maxBodyBytes)
	}
	return err
}

func RespondOK(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusOK, types.Success(data))
}

func RespondAccepted(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusAccepted, types.Success(data))
}

func RespondOKWithComparison(w http.ResponseWriter, data, comparison any) {
	WriteJSON(w, http.StatusOK, types.SuccessWithComparison(data, comparison))
}

// RespondErrorWithCause writes a failure envelope. Server errors and any
// response carrying a cause are logged; plain client errors are not.
func RespondErrorWithCause(w http.ResponseWriter, r *http.Request, status int, code, msg string, err error) {
	// Budget violations are client-fixable: remap to a typed 422 so the
	// UI can prompt narrowing instead of showing a generic 500.
	if errors.Is(err, errorcode.ErrQueryBudgetExceeded) {
		status = http.StatusUnprocessableEntity
		code = errorcode.QueryBudgetExceeded
		msg = "query exceeded its execution budget; narrow the time range or filters"
	}
	requestID := w.Header().Get("X-Request-Id")
	if err != nil || status >= http.StatusInternalServerError {
		attrs := []any{
			slog.String("code", code), slog.String("msg", msg),
			slog.String("method", r.Method), slog.String("path", r.URL.Path),
			slog.String("request_id", requestID),
		}
		if err != nil {
			attrs = append(attrs, slog.Any("error", err))
		}
		slog.ErrorContext(r.Context(), "request error", attrs...)
	}
	WriteJSON(w, status, types.Failure(code, msg, r.URL.Path, requestID))
}

// RespondServiceError maps the shared errorcode error kinds to their HTTP
// status; any other error becomes a failMsg 500.
func RespondServiceError(w http.ResponseWriter, r *http.Request, err error, failMsg string) {
	var (
		nf errorcode.NotFoundError
		cf errorcode.ConflictError
		ve errorcode.ValidationError
		ua errorcode.UnauthorizedError
		te errorcode.TrialExpiredError
		rl errorcode.RateLimitedError
	)
	switch {
	case errors.As(err, &nf):
		RespondErrorWithCause(w, r, http.StatusNotFound, errorcode.NotFound, nf.Msg, nil)
	case errors.As(err, &cf):
		RespondErrorWithCause(w, r, http.StatusConflict, errorcode.Conflict, cf.Msg, nil)
	case errors.As(err, &ve):
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, ve.Msg, nil)
	case errors.As(err, &ua):
		RespondErrorWithCause(w, r, http.StatusUnauthorized, errorcode.Unauthorized, ua.Msg, nil)
	case errors.As(err, &te):
		RespondErrorWithCause(w, r, http.StatusPaymentRequired, errorcode.TrialExpired, te.Msg, nil)
	case errors.As(err, &rl):
		RespondErrorWithCause(w, r, http.StatusTooManyRequests, errorcode.RateLimited, rl.Msg, nil)
	default:
		RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, failMsg, err)
	}
}

func ParseIDParam(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	if err != nil || id <= 0 {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "invalid "+key, nil)
		return 0, false
	}
	return id, true
}

// QueryInt64 reads an optional integer query param; an absent param yields
// def and a malformed one answers 400.
func QueryInt64(w http.ResponseWriter, r *http.Request, key string, def int64) (int64, bool) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def, true
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, key+" must be an integer", nil)
		return 0, false
	}
	return v, true
}

// QueryInt is QueryInt64 for int params.
func QueryInt(w http.ResponseWriter, r *http.Request, key string, def int) (int, bool) {
	v, ok := QueryInt64(w, r, key, int64(def))
	return int(v), ok
}

// QueryEnum reads an optional query param that must be one of allowed; an
// absent param yields "".
func QueryEnum(w http.ResponseWriter, r *http.Request, key string, allowed []string) (string, bool) {
	v := r.URL.Query().Get(key)
	if v != "" && !slices.Contains(allowed, v) {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation,
			key+" must be one of "+strings.Join(allowed, ", "), nil)
		return "", false
	}
	return v, true
}

// QueryEnums reads a repeatable query param whose every value must be one
// of allowed; an absent param yields nil.
func QueryEnums(w http.ResponseWriter, r *http.Request, key string, allowed []string) ([]string, bool) {
	values := r.URL.Query()[key]
	for _, v := range values {
		if !slices.Contains(allowed, v) {
			RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation,
				key+" must be one of "+strings.Join(allowed, ", "), nil)
			return nil, false
		}
	}
	return values, true
}

// QueryBool reads an optional true/false query param; an absent param yields
// nil.
func QueryBool(w http.ResponseWriter, r *http.Request, key string) (*bool, bool) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, true
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, key+" must be true or false", nil)
		return nil, false
	}
	return &v, true
}

// QueryLimit reads the optional limit query param under filterutil.Limit's
// rules, answering 400 when it is out of range.
func QueryLimit(w http.ResponseWriter, r *http.Request, def, max int) (int, bool) {
	raw, ok := QueryInt(w, r, "limit", 0)
	if !ok {
		return 0, false
	}
	limit, err := filterutil.Limit(raw, def, max)
	if err != nil {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, err.Error(), nil)
		return 0, false
	}
	return limit, true
}

// ParseRequiredRange reads the startTime/endTime query params as Unix
// milliseconds and answers 400 when the window is missing, inverted or
// longer than 30 days.
func ParseRequiredRange(w http.ResponseWriter, r *http.Request) (startMs, endMs int64, ok bool) {
	return requireRange(w, r, filterutil.MaxTimeRangeMs)
}

// ParseRequiredUncappedRange is ParseRequiredRange without the 30-day cap.
func ParseRequiredUncappedRange(w http.ResponseWriter, r *http.Request) (startMs, endMs int64, ok bool) {
	return requireRange(w, r, 0)
}

func requireRange(w http.ResponseWriter, r *http.Request, maxMs int64) (startMs, endMs int64, ok bool) {
	startMs, endMs, err := parseRange(r, maxMs)
	if err != nil {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, err.Error(), nil)
		return 0, 0, false
	}
	return startMs, endMs, true
}

// parseRange validates the window; maxMs of 0 leaves its length unbounded.
func parseRange(r *http.Request, maxMs int64) (startMs, endMs int64, err error) {
	q := r.URL.Query()
	startMs, startErr := strconv.ParseInt(q.Get("startTime"), 10, 64)
	endMs, endErr := strconv.ParseInt(q.Get("endTime"), 10, 64)
	switch {
	case startErr != nil || endErr != nil || startMs <= 0 || endMs <= 0:
		return 0, 0, errors.New("startTime and endTime must be positive Unix milliseconds")
	case startMs >= endMs:
		return 0, 0, errors.New("startTime must be before endTime")
	case maxMs > 0 && endMs-startMs > maxMs:
		return 0, 0, errors.New("time range must not exceed 30 days")
	}
	return startMs, endMs, nil
}

// comparisonShift returns how far before the requested window the compareTo
// preset places the comparison window; 0 means no comparison was asked for.
func comparisonShift(preset string, windowMs int64) (int64, error) {
	switch preset {
	case "":
		return 0, nil
	case "previous_period":
		return windowMs, nil
	case "previous_day":
		return (24 * time.Hour).Milliseconds(), nil
	case "previous_week":
		return (7 * 24 * time.Hour).Milliseconds(), nil
	default:
		return 0, errors.New("compareTo must be previous_period, previous_day or previous_week")
	}
}

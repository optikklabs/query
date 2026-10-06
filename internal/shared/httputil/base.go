package httputil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
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

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
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
		un errorcode.UnavailableError
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
	case errors.As(err, &un):
		RespondErrorWithCause(w, r, http.StatusServiceUnavailable, errorcode.Unavailable, un.Msg, nil)
	default:
		RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, failMsg, err)
	}
}

func ParseInt64Param(r *http.Request, key string, fallback int64) int64 {
	if v := r.URL.Query().Get(key); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func ParseIDParam(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	if err != nil || id <= 0 {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "invalid "+key, nil)
		return 0, false
	}
	return id, true
}

const MaxPageSize = 200

func ParseIntParam(r *http.Request, key string, fallback int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return fallback
}

func ParsePageSize(r *http.Request, key string, fallback int) int {
	size := min(ParseIntParam(r, key, fallback), MaxPageSize)
	if size <= 0 {
		size = fallback
	}
	return size
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
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, err.Error(), nil)
		return 0, 0, false
	}
	return startMs, endMs, true
}

// parseRange validates the window; maxMs of 0 leaves its length unbounded.
func parseRange(r *http.Request, maxMs int64) (startMs, endMs int64, err error) {
	startMs = ParseInt64Param(r, "startTime", 0)
	endMs = ParseInt64Param(r, "endTime", 0)
	switch {
	case startMs <= 0 || endMs <= 0:
		return 0, 0, errors.New("startTime and endTime must be positive Unix milliseconds")
	case startMs >= endMs:
		return 0, 0, errors.New("startTime must be before endTime")
	case maxMs > 0 && endMs-startMs > maxMs:
		return 0, 0, errors.New("time range must not exceed 30 days")
	}
	return startMs, endMs, nil
}

// ParseComparisonRange resolves the comparison window from explicit
// compareStart/compareEnd params or a compareTo preset.
func ParseComparisonRange(r *http.Request, startMs, endMs int64) (cmpStart, cmpEnd int64, ok bool) {
	cmpStart = ParseInt64Param(r, "compareStart", 0)
	cmpEnd = ParseInt64Param(r, "compareEnd", 0)
	if cmpStart > 0 && cmpEnd > 0 {
		return cmpStart, cmpEnd, true
	}

	var shift int64
	switch r.URL.Query().Get("compareTo") {
	case "previous_period":
		shift = endMs - startMs
	case "previous_day":
		shift = (24 * time.Hour).Milliseconds()
	case "previous_week":
		shift = (7 * 24 * time.Hour).Milliseconds()
	default:
		return 0, 0, false
	}
	return startMs - shift, endMs - shift, true
}

package httputil

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	types "github.com/optikklabs/query/internal/shared/contracts"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

const APIV1Base = "/api/v1"

func Tenant(r *http.Request) types.TenantContext {
	return types.TenantFrom(r.Context())
}

func URLParamLower(r *http.Request, key string) string {
	return strings.ToLower(chi.URLParam(r, key))
}

func ClientIP(r *http.Request) string {
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

func DecodeJSON(r *http.Request, v any) error {
	const maxBodyBytes = 1 << 20
	limited := &io.LimitedReader{R: r.Body, N: maxBodyBytes + 1}
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	if limited.N <= 0 {
		return errors.New("request body exceeds 1 MiB")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain one JSON value")
		}
		return err
	}
	if limited.N <= 0 {
		return errors.New("request body exceeds 1 MiB")
	}
	return nil
}

func RespondOK(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusOK, types.Success(data))
}

func RespondErrorWithCause(w http.ResponseWriter, r *http.Request, status int, code, msg string, err error) {
	// Budget violations are client-fixable: remap to a typed 422 so the
	// UI can prompt narrowing instead of showing a generic 500.
	if err != nil && errors.Is(err, errorcode.ErrQueryBudgetExceeded) {
		status = http.StatusUnprocessableEntity
		code = errorcode.QueryBudgetExceeded
		msg = "query exceeded its execution budget; narrow the time range or filters"
	}
	requestID := w.Header().Get("X-Request-Id")
	if err != nil {
		slog.ErrorContext(r.Context(), "request error",
			slog.String("code", code), slog.String("msg", msg),
			slog.String("method", r.Method), slog.String("path", r.URL.Path),
			slog.String("request_id", requestID),
			slog.Any("error", err))
	} else if status >= 500 {
		slog.ErrorContext(r.Context(), "request error",
			slog.String("code", code), slog.String("msg", msg),
			slog.String("method", r.Method), slog.String("path", r.URL.Path),
			slog.String("request_id", requestID))
	}
	WriteJSON(w, status, types.Failure(code, msg, r.URL.Path, requestID))
}

// RespondServiceError maps the shared service error kinds (validation,
// not-found, conflict) to HTTP responses; other errors become failMsg 500s.
func RespondServiceError(w http.ResponseWriter, r *http.Request, err error, failMsg string) {
	var (
		nf errorcode.NotFoundError
		cf errorcode.ConflictError
		ve errorcode.ValidationError
	)
	switch {
	case errors.As(err, &nf):
		RespondErrorWithCause(w, r, http.StatusNotFound, errorcode.NotFound, nf.Msg, nil)
	case errors.As(err, &cf):
		RespondErrorWithCause(w, r, http.StatusConflict, errorcode.Conflict, cf.Msg, nil)
	case errors.As(err, &ve):
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, ve.Msg, nil)
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
	size := ParseIntParam(r, key, fallback)
	if size > MaxPageSize {
		size = MaxPageSize
	}
	if size <= 0 {
		size = fallback
	}
	return size
}

// Allowance for client clocks slightly ahead of the server.
const rangeClockSkewMs = 60 * 1000

func ParseRange(r *http.Request) (startMs, endMs int64, err error) {
	now := time.Now().UnixMilli()
	end := ParseInt64Param(r, "endTime", 0)
	if end <= 0 {
		end = ParseInt64Param(r, "end", 0)
	}
	start := ParseInt64Param(r, "startTime", 0)
	if start <= 0 {
		start = ParseInt64Param(r, "start", 0)
	}
	if end <= 0 {
		end = now
	}
	if start <= 0 {
		start = end - (7 * 24 * 3600 * 1000)
	}
	if maxEnd := now + rangeClockSkewMs; end > maxEnd {
		end = maxEnd
	}
	if end-start > filterutil.MaxTimeRangeMs {
		start = end - filterutil.MaxTimeRangeMs
	}
	if start >= end {
		return 0, 0, errors.New("start must be before end")
	}
	return start, end, nil
}

func ParseRequiredRange(w http.ResponseWriter, r *http.Request) (startMs, endMs int64, ok bool) {
	q := r.URL.Query()
	hasStart := q.Get("startTime") != "" || q.Get("start") != ""
	hasEnd := q.Get("endTime") != "" || q.Get("end") != ""
	if !hasStart || !hasEnd {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "start and end time params are required", nil)
		return 0, 0, false
	}
	start, end, err := ParseRange(r)
	if err != nil {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "start and end time params are required", err)
		return 0, 0, false
	}
	return start, end, true
}

func ParseRequiredExplicitRange(w http.ResponseWriter, r *http.Request) (startMs, endMs int64, ok bool) {
	startRaw := r.URL.Query().Get("startTime")
	endRaw := r.URL.Query().Get("endTime")
	if startRaw == "" || endRaw == "" {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "startTime and endTime are required", nil)
		return 0, 0, false
	}

	start, startErr := strconv.ParseInt(startRaw, 10, 64)
	end, endErr := strconv.ParseInt(endRaw, 10, 64)
	if startErr != nil || endErr != nil || start <= 0 || end <= 0 || start >= end {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.BadRequest, "startTime and endTime must be positive Unix milliseconds with startTime before endTime", nil)
		return 0, 0, false
	}
	return start, end, true
}

func ParseComparisonRange(r *http.Request, startMs, endMs int64) (cmpStart, cmpEnd int64, ok bool) {
	cmpStart = ParseInt64Param(r, "compareStart", 0)
	cmpEnd = ParseInt64Param(r, "compareEnd", 0)
	if cmpStart > 0 && cmpEnd > 0 {
		return cmpStart, cmpEnd, true
	}

	compareTo := r.URL.Query().Get("compareTo")
	duration := endMs - startMs
	switch compareTo {
	case "previous_period":
		return startMs - duration, startMs, true
	case "previous_day":
		return startMs - 86400000, endMs - 86400000, true
	case "previous_week":
		return startMs - 604800000, endMs - 604800000, true
	default:
		return 0, 0, false
	}
}

func RespondOKWithComparison(w http.ResponseWriter, data, comparison any) {
	WriteJSON(w, http.StatusOK, types.SuccessWithComparison(data, comparison))
}

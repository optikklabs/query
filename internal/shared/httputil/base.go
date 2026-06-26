// Package httputil contains shared HTTP handler helpers.
package httputil

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	types "github.com/optikklabs/query/internal/shared/contracts"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

var validate = validator.New()

func Tenant(r *http.Request) types.TenantContext {
	return types.TenantFrom(r.Context())
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
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return err
	}
	if reflect.Indirect(reflect.ValueOf(v)).Kind() == reflect.Struct {
		return validate.Struct(v)
	}
	return nil
}

func RespondOK(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusOK, types.Success(data))
}

func RespondError(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	if status >= 500 {
		slog.Error("request error",
			slog.String("code", code), slog.String("msg", msg),
			slog.String("method", r.Method), slog.String("path", r.URL.Path))
	}
	WriteJSON(w, status, types.Failure(code, msg, r.URL.Path))
}

func RespondErrorWithCause(w http.ResponseWriter, r *http.Request, status int, code, msg string, err error) {
	if err != nil {
		slog.Error("request error",
			slog.String("code", code), slog.String("msg", msg),
			slog.String("method", r.Method), slog.String("path", r.URL.Path),
			slog.Any("error", err))
		msg = fmt.Sprintf("%s: %s", msg, dbutil.SanitizeError(err))
	} else if status >= 500 {
		slog.Error("request error",
			slog.String("code", code), slog.String("msg", msg),
			slog.String("method", r.Method), slog.String("path", r.URL.Path))
	}
	WriteJSON(w, status, types.Failure(code, msg, r.URL.Path))
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

func ParseFloatParam(r *http.Request, key string, fallback float64) float64 {
	if v := r.URL.Query().Get(key); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
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
		return 0, 0, errors.New("start time is required")
	}
	if start >= end {
		return 0, 0, errors.New("start must be before end")
	}
	return start, end, nil
}

func ParseRequiredRange(w http.ResponseWriter, r *http.Request) (startMs, endMs int64, ok bool) {
	start, end, err := ParseRange(r)
	if err != nil {
		RespondError(w, r, http.StatusBadRequest, errorcode.BadRequest, "start and end time params are required")
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

type ComparisonResponse struct {
	Data       any `json:"data"`
	Comparison any `json:"comparison,omitempty"`
}

func WithComparison(r *http.Request, startMs, endMs int64, queryFn func(s, e int64) (any, error)) (ComparisonResponse, error) {
	cmpStart, cmpEnd, hasCmp := ParseComparisonRange(r, startMs, endMs)

	if !hasCmp {
		primary, err := queryFn(startMs, endMs)
		if err != nil {
			return ComparisonResponse{}, err
		}
		return ComparisonResponse{Data: primary}, nil
	}

	var (
		primary, comparison any
		primaryErr, cmpErr  error
		wg                  sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		primary, primaryErr = queryFn(startMs, endMs)
	}()
	go func() {
		defer wg.Done()
		comparison, cmpErr = queryFn(cmpStart, cmpEnd)
	}()
	wg.Wait()

	if primaryErr != nil {
		return ComparisonResponse{}, primaryErr
	}

	resp := ComparisonResponse{Data: primary}
	if cmpErr == nil {
		resp.Comparison = comparison
	}
	return resp, nil
}

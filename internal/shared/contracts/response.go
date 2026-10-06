package contracts

import (
	"reflect"
	"time"
)

type APIResponse struct {
	Success bool `json:"success"`
	Data    any  `json:"data,omitempty"`

	Comparison any          `json:"comparison,omitempty"`
	Error      *ErrorDetail `json:"error,omitempty"`
	Timestamp  time.Time    `json:"timestamp"`
}

type ErrorDetail struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Path      string    `json:"path,omitempty"`
	RequestID string    `json:"requestId,omitempty"`
}

// PageInfo describes cursor-based pagination state for list endpoints.
type PageInfo struct {
	HasMore    bool   `json:"hasMore"`
	NextCursor string `json:"nextCursor,omitempty"`
	Limit      int    `json:"limit"`
}

func Success(data any) APIResponse {
	return APIResponse{Success: true, Data: emptyIfNilSlice(data), Timestamp: time.Now().UTC()}
}

func SuccessWithComparison(data, comparison any) APIResponse {
	return APIResponse{
		Success:    true,
		Data:       emptyIfNilSlice(data),
		Comparison: emptyIfNilSlice(comparison),
		Timestamp:  time.Now().UTC(),
	}
}

func Failure(code, msg, path, requestID string) APIResponse {
	now := time.Now().UTC()
	return APIResponse{
		Success:   false,
		Error:     &ErrorDetail{Code: code, Message: msg, Timestamp: now, Path: path, RequestID: requestID},
		Timestamp: now,
	}
}

// emptyIfNilSlice turns a nil slice into an empty one of the same type, so a
// list endpoint with no rows encodes [] instead of null.
func emptyIfNilSlice(data any) any {
	if v := reflect.ValueOf(data); v.Kind() == reflect.Slice && v.IsNil() {
		return reflect.MakeSlice(v.Type(), 0, 0).Interface()
	}
	return data
}

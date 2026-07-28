package contracts

import "time"

type APIResponse struct {
	Success bool `json:"success"`
	Data    any  `json:"data,omitempty"`

	Comparison any          `json:"comparison,omitempty"`
	Error      *ErrorDetail `json:"error,omitempty"`
	Timestamp  time.Time    `json:"timestamp"`
}

type ErrorDetail struct {
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	Timestamp   time.Time         `json:"timestamp"`
	Path        string            `json:"path,omitempty"`
	RequestID   string            `json:"requestId,omitempty"`
	FieldErrors map[string]string `json:"fieldErrors,omitempty"`
}

// PageInfo describes cursor-based pagination state for list endpoints.
type PageInfo struct {
	HasMore    bool   `json:"hasMore"`
	NextCursor string `json:"nextCursor,omitempty"`
	Limit      int    `json:"limit"`
}

func Success(data any) APIResponse {
	return APIResponse{Success: true, Data: data, Timestamp: time.Now().UTC()}
}

func SuccessWithComparison(data, comparison any) APIResponse {
	return APIResponse{Success: true, Data: data, Comparison: comparison, Timestamp: time.Now().UTC()}
}

func Failure(code, msg, path string, requestID ...string) APIResponse {
	detail := &ErrorDetail{
		Code:      code,
		Message:   msg,
		Timestamp: time.Now().UTC(),
		Path:      path,
	}
	if len(requestID) > 0 {
		detail.RequestID = requestID[0]
	}
	return APIResponse{
		Success:   false,
		Error:     detail,
		Timestamp: time.Now().UTC(),
	}
}

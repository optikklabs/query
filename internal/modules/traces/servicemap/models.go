package servicemap

import (
	"time"
)

type TraceErrorGroup struct {
	ExceptionType string           `json:"exceptionType"`
	Count         int              `json:"count"`
	Spans         []TraceErrorSpan `json:"spans"`
}

type TraceErrorSpan struct {
	SpanID           string    `json:"spanId"`
	ServiceName      string    `json:"serviceName"`
	OperationName    string    `json:"operationName"`
	ExceptionMessage string    `json:"exceptionMessage,omitempty"`
	StatusMessage    string    `json:"statusMessage,omitempty"`
	StartTime        time.Time `json:"startTime"`
	DurationMs       float64   `json:"durationMs"`
}

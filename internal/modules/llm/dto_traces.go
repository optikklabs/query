package llm

import (
	"time"

	"github.com/optikklabs/query/internal/shared/contracts"
)

type TracesQueryRequest struct {
	StartTime     int64    `json:"startTime"`
	EndTime       int64    `json:"endTime"`
	Limit         int      `json:"limit"`
	Cursor        string   `json:"cursor"`
	Services      []string `json:"services"`
	Vendors       []string `json:"vendors"`
	Models        []string `json:"models"`
	Status        string   `json:"status"`
	MinDurationMs float64  `json:"minDurationMs"`
}

type TracesQueryResponse struct {
	Results  []LLMTrace         `json:"results"`
	PageInfo contracts.PageInfo `json:"pageInfo"`
}

type LLMTrace struct {
	TraceID       string       `json:"traceId"`
	StartMs       int64        `json:"startMs"`
	DurationMs    float64      `json:"durationMs"`
	Service       string       `json:"service"`
	Operation     string       `json:"operation"`
	Status        string       `json:"status"`
	HasError      bool         `json:"hasError"`
	Level         string       `json:"level"`
	Vendor        string       `json:"vendor"`
	Model         string       `json:"model"`
	UserID        string       `json:"userId"`
	SessionID     string       `json:"sessionId"`
	Tags          []string     `json:"tags"`
	LLMCalls      uint64       `json:"llmCalls"`
	PromptPreview string       `json:"promptPreview"`
	InputTokens   uint64       `json:"inputTokens"`
	OutputTokens  uint64       `json:"outputTokens"`
	Cost          float64      `json:"cost"`
	Scores        []TraceScore `json:"scores"`
}

type llmTraceRow struct {
	TraceID       string    `ch:"trace_id"`
	SpanID        string    `ch:"span_id"`
	StartTime     time.Time `ch:"start_time"`
	DurationNano  uint64    `ch:"duration_nano"`
	Service       string    `ch:"service"`
	Operation     string    `ch:"operation"`
	Status        string    `ch:"status"`
	HasError      bool      `ch:"has_error"`
	Vendor        string    `ch:"vendor"`
	Model         string    `ch:"model"`
	UserID        string    `ch:"user_id"`
	SessionID     string    `ch:"session_id"`
	Tags          []string  `ch:"tags"`
	LLMCalls      uint64    `ch:"llm_calls"`
	PromptPreview string    `ch:"prompt_preview"`
	InputTokens   uint64    `ch:"input_tokens"`
	OutputTokens  uint64    `ch:"output_tokens"`
	Cost          float64   `ch:"cost"`
}

type TraceScore struct {
	Name     string  `json:"name"`
	DataType string  `json:"dataType"`
	Value    float64 `json:"value"`
	String   string  `json:"stringValue,omitempty"`
	Source   string  `json:"source"`
	Comment  string  `json:"comment,omitempty"`
}

type traceScoreRow struct {
	TraceID  string  `ch:"trace_id"`
	Name     string  `ch:"name"`
	DataType string  `ch:"data_type"`
	Value    float64 `ch:"value"`
	String   string  `ch:"string_value"`
	Source   string  `ch:"source"`
	Comment  string  `ch:"comment"`
}

type traceCursor struct {
	StartNs uint64 `json:"s"`
	SpanID  string `json:"i"`
}

type TraceDetailResponse struct {
	TraceID      string       `json:"traceId"`
	Name         string       `json:"name"`
	Service      string       `json:"service"`
	Environment  string       `json:"environment"`
	UserID       string       `json:"userId"`
	SessionID    string       `json:"sessionId"`
	Release      string       `json:"release"`
	StartMs      int64        `json:"startMs"`
	DurationMs   float64      `json:"durationMs"`
	HasError     bool         `json:"hasError"`
	Prompt       string       `json:"prompt"`
	Output       string       `json:"output"`
	InputTokens  uint64       `json:"inputTokens"`
	OutputTokens uint64       `json:"outputTokens"`
	Cost         float64      `json:"cost"`
	Spans        []LLMSpan    `json:"spans"`
	Scores       []TraceScore `json:"scores"`
}

type LLMSpan struct {
	SpanID        string  `json:"spanId"`
	ParentSpanID  string  `json:"parentSpanId"`
	Name          string  `json:"name"`
	Service       string  `json:"service"`
	Operation     string  `json:"operation"`
	Kind          string  `json:"kind"`
	Vendor        string  `json:"vendor"`
	Model         string  `json:"model"`
	ResponseModel string  `json:"responseModel,omitempty"`
	StartMs       int64   `json:"startMs"`
	DurationMs    float64 `json:"durationMs"`
	HasError      bool    `json:"hasError"`
	InputTokens   uint64  `json:"inputTokens"`
	OutputTokens  uint64  `json:"outputTokens"`
	Cost          float64 `json:"cost"`
	Prompt        string  `json:"prompt,omitempty"`
	Completion    string  `json:"completion,omitempty"`
	// True when the text was truncated server-side; fetch the full
	// content via GET /llm/traces/{traceId}/spans/{spanId}/io.
	PromptTruncated     bool `json:"promptTruncated,omitempty"`
	CompletionTruncated bool `json:"completionTruncated,omitempty"`
}

type SpanIOResponse struct {
	TraceID    string `json:"traceId"`
	SpanID     string `json:"spanId"`
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
}

type traceSpanRow struct {
	SpanID              string    `ch:"span_id"`
	ParentSpanID        string    `ch:"parent_span_id"`
	Timestamp           time.Time `ch:"timestamp"`
	DurationNano        uint64    `ch:"duration_nano"`
	Name                string    `ch:"name"`
	Service             string    `ch:"service"`
	Environment         string    `ch:"environment"`
	Vendor              string    `ch:"gen_ai_system"`
	Operation           string    `ch:"gen_ai_operation"`
	Kind                string    `ch:"gen_ai_span_kind"`
	Model               string    `ch:"gen_ai_request_model"`
	ResponseModel       string    `ch:"gen_ai_response_model"`
	InputTokens         uint64    `ch:"gen_ai_input_tokens"`
	OutputTokens        uint64    `ch:"gen_ai_output_tokens"`
	HasError            bool      `ch:"has_error"`
	UserID              string    `ch:"llm_user_id"`
	SessionID           string    `ch:"llm_session_id"`
	Release             string    `ch:"llm_release"`
	Prompt              string    `ch:"prompt"`
	Completion          string    `ch:"completion"`
	PromptTruncated     uint8     `ch:"prompt_truncated"`
	CompletionTruncated uint8     `ch:"completion_truncated"`
}

type spanIORow struct {
	Prompt     string `ch:"prompt"`
	Completion string `ch:"completion"`
}

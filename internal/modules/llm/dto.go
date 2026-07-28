package llm

import "time"

type App struct {
	Service        string   `json:"service"`
	Kind           string   `json:"kind"`
	Vendor         string   `json:"vendor"`
	PrimaryModel   string   `json:"primaryModel"`
	LLMSpans       uint64   `json:"llmSpans"`
	ToolSpans      uint64   `json:"toolSpans"`
	RetrievalSpans uint64   `json:"retrievalSpans"`
	EmbeddingSpans uint64   `json:"embeddingSpans"`
	AgentSpans     uint64   `json:"agentSpans"`
	TotalSpans     uint64   `json:"totalSpans"`
	ErrorRate      float64  `json:"errorRate"`
	P50Ms          float64  `json:"p50Ms"`
	P95Ms          float64  `json:"p95Ms"`
	P99Ms          float64  `json:"p99Ms"`
	InputTokens    uint64   `json:"inputTokens"`
	OutputTokens   uint64   `json:"outputTokens"`
	Cost           float64  `json:"cost"`
	Trend          []uint64 `json:"trend"`
}

type AppsResponse struct {
	Apps []App `json:"apps"`
}

type appAggRow struct {
	Service        string    `ch:"service"`
	LLMSpans       uint64    `ch:"llm_spans"`
	ToolSpans      uint64    `ch:"tool_spans"`
	RetrievalSpans uint64    `ch:"retrieval_spans"`
	EmbeddingSpans uint64    `ch:"embedding_spans"`
	AgentSpans     uint64    `ch:"agent_spans"`
	TotalSpans     uint64    `ch:"total_spans"`
	ErrorSpans     uint64    `ch:"error_spans"`
	InputTokens    uint64    `ch:"in_tokens"`
	OutputTokens   uint64    `ch:"out_tokens"`
	QS             []float64 `ch:"qs"`
	Cost           float64   `ch:"cost"`
}

type modelBreakdownRow struct {
	Service      string  `ch:"service"`
	Vendor       string  `ch:"vendor"`
	Model        string  `ch:"model"`
	Spans        uint64  `ch:"spans"`
	InputTokens  uint64  `ch:"in_tokens"`
	OutputTokens uint64  `ch:"out_tokens"`
	Cost         float64 `ch:"cost"`
}

type trendRow struct {
	BucketAt time.Time `ch:"bucket_at"`
	Service  string    `ch:"service"`
	Count    uint64    `ch:"cnt"`
}

type TimeseriesResponse struct {
	Series []Series `json:"series"`
}

type Series struct {
	Key    string  `json:"key"`
	Points []Point `json:"points"`
}

type Point struct {
	T     int64   `json:"t"`
	Value float64 `json:"value"`
}

type keyedBucketRow struct {
	BucketAt time.Time `ch:"bucket_at"`
	Key      string    `ch:"key"`
	Value    float64   `ch:"value"`
}

type latencyBucketRow struct {
	BucketAt time.Time `ch:"bucket_at"`
	QS       []float64 `ch:"qs"`
}

type ModelUsage struct {
	Model        string  `json:"model"`
	Vendor       string  `json:"vendor"`
	Traces       uint64  `json:"traces"`
	InputTokens  uint64  `json:"inputTokens"`
	OutputTokens uint64  `json:"outputTokens"`
	P50Ms        float64 `json:"p50Ms"`
	P95Ms        float64 `json:"p95Ms"`
	Cost         float64 `json:"cost"`
}

type ModelsResponse struct {
	Models []ModelUsage `json:"models"`
}

type modelUsageRow struct {
	Model        string    `ch:"model"`
	Vendor       string    `ch:"vendor"`
	Traces       uint64    `ch:"traces"`
	InputTokens  uint64    `ch:"in_tokens"`
	OutputTokens uint64    `ch:"out_tokens"`
	QS           []float64 `ch:"qs"`
	Cost         float64   `ch:"cost"`
}

type CostBreakdownResponse struct {
	GroupBy string          `json:"groupBy"`
	Rows    []CostBreakdown `json:"rows"`
}

type CostBreakdown struct {
	Key          string  `json:"key"`
	Vendor       string  `json:"vendor,omitempty"`
	LLMSpans     uint64  `json:"llmSpans"`
	InputTokens  uint64  `json:"inputTokens"`
	OutputTokens uint64  `json:"outputTokens"`
	Cost         float64 `json:"cost"`
}

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
	Results  []LLMTrace `json:"results"`
	PageInfo PageInfo   `json:"pageInfo"`
}

type PageInfo struct {
	HasMore    bool   `json:"hasMore"`
	NextCursor string `json:"nextCursor,omitempty"`
	Limit      int    `json:"limit"`
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
}

type OverviewResponse struct {
	Current  OverviewWindow `json:"current"`
	Previous OverviewWindow `json:"previous"`
	Series   OverviewSeries `json:"series"`
}

type OverviewWindow struct {
	LLMSpans     uint64  `json:"llmSpans"`
	ToolSpans    uint64  `json:"toolSpans"`
	TotalSpans   uint64  `json:"totalSpans"`
	Traces       uint64  `json:"traces"`
	InputTokens  uint64  `json:"inputTokens"`
	OutputTokens uint64  `json:"outputTokens"`
	ErrorRate    float64 `json:"errorRate"`
	P50Ms        float64 `json:"p50Ms"`
	P95Ms        float64 `json:"p95Ms"`
	P99Ms        float64 `json:"p99Ms"`
	Cost         float64 `json:"cost"`
}

type OverviewSeries struct {
	Timestamps []int64   `json:"timestamps"`
	LLMSpans   []uint64  `json:"llmSpans"`
	ToolSpans  []uint64  `json:"toolSpans"`
	ErrorRate  []float64 `json:"errorRate"`
	P95Ms      []float64 `json:"p95Ms"`
	Cost       []float64 `json:"cost"`
}

type overviewWindowRow struct {
	IsCurrent    uint8     `ch:"is_current"`
	LLMSpans     uint64    `ch:"llm_spans"`
	ToolSpans    uint64    `ch:"tool_spans"`
	TotalSpans   uint64    `ch:"total_spans"`
	ErrorSpans   uint64    `ch:"error_spans"`
	InputTokens  uint64    `ch:"in_tokens"`
	OutputTokens uint64    `ch:"out_tokens"`
	QS           []float64 `ch:"qs"`
	Cost         float64   `ch:"cost"`
}

type overviewSeriesRow struct {
	BucketAt   time.Time `ch:"bucket_at"`
	LLMSpans   uint64    `ch:"llm_spans"`
	ToolSpans  uint64    `ch:"tool_spans"`
	TotalSpans uint64    `ch:"total_spans"`
	ErrorSpans uint64    `ch:"error_spans"`
	QS         []float64 `ch:"qs"`
	Cost       float64   `ch:"cost"`
}

type traceCountRow struct {
	IsCurrent uint8  `ch:"is_current"`
	Traces    uint64 `ch:"traces"`
	Spans     uint64 `ch:"spans"`
}

type traceSpanRow struct {
	SpanID        string    `ch:"span_id"`
	ParentSpanID  string    `ch:"parent_span_id"`
	Timestamp     time.Time `ch:"timestamp"`
	DurationNano  uint64    `ch:"duration_nano"`
	Name          string    `ch:"name"`
	Service       string    `ch:"service"`
	Environment   string    `ch:"environment"`
	Vendor        string    `ch:"gen_ai_system"`
	Operation     string    `ch:"gen_ai_operation"`
	Kind          string    `ch:"gen_ai_span_kind"`
	Model         string    `ch:"gen_ai_request_model"`
	ResponseModel string    `ch:"gen_ai_response_model"`
	InputTokens   uint64    `ch:"gen_ai_input_tokens"`
	OutputTokens  uint64    `ch:"gen_ai_output_tokens"`
	HasError      bool      `ch:"has_error"`
	UserID        string    `ch:"llm_user_id"`
	SessionID     string    `ch:"llm_session_id"`
	Release       string    `ch:"llm_release"`
	Prompt        string    `ch:"prompt"`
	Completion    string    `ch:"completion"`
}

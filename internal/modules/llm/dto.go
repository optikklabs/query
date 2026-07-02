package llm

import "time"

// App is one LLM-emitting service with rollup aggregates for the window.
type App struct {
	Service        string   `json:"service"`
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

// TimeseriesResponse pivots rollup rows into one series per key.
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

// CostBreakdownResponse groups spend by service, vendor or model.
type CostBreakdownResponse struct {
	GroupBy string           `json:"groupBy"`
	Rows    []CostBreakdown  `json:"rows"`
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
	TraceID      string  `json:"traceId"`
	StartMs      int64   `json:"startMs"`
	DurationMs   float64 `json:"durationMs"`
	Service      string  `json:"service"`
	Operation    string  `json:"operation"`
	Status       string  `json:"status"`
	HasError     bool    `json:"hasError"`
	Vendor       string  `json:"vendor"`
	Model        string  `json:"model"`
	LLMCalls     uint64  `json:"llmCalls"`
	InputTokens  uint64  `json:"inputTokens"`
	OutputTokens uint64  `json:"outputTokens"`
	Cost         float64 `json:"cost"`
}

type llmTraceRow struct {
	TraceID      string    `ch:"trace_id"`
	SpanID       string    `ch:"span_id"`
	StartTime    time.Time `ch:"start_time"`
	DurationNano uint64    `ch:"duration_nano"`
	Service      string    `ch:"service"`
	Operation    string    `ch:"operation"`
	Status       string    `ch:"status"`
	HasError     bool      `ch:"has_error"`
	Vendor       string    `ch:"vendor"`
	Model        string    `ch:"model"`
	LLMCalls     uint64    `ch:"llm_calls"`
	InputTokens  uint64    `ch:"input_tokens"`
	OutputTokens uint64    `ch:"output_tokens"`
	Cost         float64   `ch:"cost"`
}

type traceCursor struct {
	StartNs uint64 `json:"s"`
	SpanID  string `json:"i"`
}

// TraceDetailResponse is the waterfall + prompt/output view of one trace.
type TraceDetailResponse struct {
	TraceID      string     `json:"traceId"`
	Service      string     `json:"service"`
	StartMs      int64      `json:"startMs"`
	DurationMs   float64    `json:"durationMs"`
	HasError     bool       `json:"hasError"`
	Prompt       string     `json:"prompt"`
	Output       string     `json:"output"`
	InputTokens  uint64     `json:"inputTokens"`
	OutputTokens uint64     `json:"outputTokens"`
	Cost         float64    `json:"cost"`
	Spans        []LLMSpan  `json:"spans"`
}

type LLMSpan struct {
	SpanID       string  `json:"spanId"`
	ParentSpanID string  `json:"parentSpanId"`
	Name         string  `json:"name"`
	Service      string  `json:"service"`
	Operation    string  `json:"operation"`
	Vendor       string  `json:"vendor"`
	Model        string  `json:"model"`
	StartMs      int64   `json:"startMs"`
	DurationMs   float64 `json:"durationMs"`
	HasError     bool    `json:"hasError"`
	InputTokens  uint64  `json:"inputTokens"`
	OutputTokens uint64  `json:"outputTokens"`
	Cost         float64 `json:"cost"`
}

type traceSpanRow struct {
	SpanID       string    `ch:"span_id"`
	ParentSpanID string    `ch:"parent_span_id"`
	Timestamp    time.Time `ch:"timestamp"`
	DurationNano uint64    `ch:"duration_nano"`
	Name         string    `ch:"name"`
	Service      string    `ch:"service"`
	Vendor       string    `ch:"gen_ai_system"`
	Operation    string    `ch:"gen_ai_operation"`
	Model        string    `ch:"gen_ai_request_model"`
	InputTokens  uint64    `ch:"gen_ai_input_tokens"`
	OutputTokens uint64    `ch:"gen_ai_output_tokens"`
	HasError     bool      `ch:"has_error"`
	Prompt       string    `ch:"prompt"`
	Completion   string    `ch:"completion"`
}

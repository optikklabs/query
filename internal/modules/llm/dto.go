package llm

import (
	"time"
)

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

type costBreakdownRow struct {
	Key          string  `ch:"key"`
	TopVendor    string  `ch:"top_vendor"`
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

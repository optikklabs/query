package llm

import (
	"time"
)

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

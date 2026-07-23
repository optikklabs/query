package playground

import "github.com/optikklabs/query/internal/infra/llmproviders"

// CompleteRequest is a single interactive playground completion.
type CompleteRequest struct {
	Provider    string                 `json:"provider"`
	Model       string                 `json:"model"`
	Messages    []llmproviders.Message `json:"messages"`
	Temperature float64                `json:"temperature"`
	MaxTokens   int                    `json:"maxTokens"`
}

// CompleteResponse returns the model output plus usage and cost.
type CompleteResponse struct {
	Output       string  `json:"output"`
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	LatencyMs    int64   `json:"latencyMs"`
	CostUsd      float64 `json:"costUsd"`
}

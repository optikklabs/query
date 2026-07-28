package llmproviders

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionRequest struct {
	Model       string
	Messages    []Message
	Temperature float64
	MaxTokens   int
}

type CompletionResult struct {
	Output       string
	InputTokens  int
	OutputTokens int
}

type Client interface {
	Complete(ctx context.Context, apiKey string, req CompletionRequest) (CompletionResult, error)
}

var ErrUnknownProvider = errors.New("unknown provider")

type Registry struct {
	clients map[string]Client
}

func NewRegistry() *Registry {
	httpc := &http.Client{Timeout: 30 * time.Second}
	return &Registry{clients: map[string]Client{
		"openai":    &openAIClient{http: httpc, baseURL: "https://api.openai.com/v1"},
		"mistral":   &openAIClient{http: httpc, baseURL: "https://api.mistral.ai/v1"},
		"anthropic": &anthropicClient{http: httpc, baseURL: "https://api.anthropic.com/v1"},
	}}
}

func (r *Registry) Complete(ctx context.Context, provider, apiKey string, req CompletionRequest) (CompletionResult, error) {
	c, ok := r.clients[provider]
	if !ok {
		return CompletionResult{}, ErrUnknownProvider
	}
	return c.Complete(ctx, apiKey, req)
}

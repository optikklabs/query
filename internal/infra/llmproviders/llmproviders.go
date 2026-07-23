// Package llmproviders holds minimal outbound clients for the LLM playground
// and dataset experiments. Base URLs are fixed (no SSRF surface); each call is
// a single request with a hard timeout and no retries — the caller owns
// concurrency limiting and budget enforcement.
package llmproviders

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Message is one turn in a chat completion.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionRequest is the provider-agnostic completion input.
type CompletionRequest struct {
	Model       string
	Messages    []Message
	Temperature float64
	MaxTokens   int
}

// CompletionResult is the normalised completion output.
type CompletionResult struct {
	Output       string
	InputTokens  int
	OutputTokens int
}

// Client performs a single completion against one provider.
type Client interface {
	Complete(ctx context.Context, apiKey string, req CompletionRequest) (CompletionResult, error)
}

// ErrUnknownProvider is returned for an unregistered provider name.
var ErrUnknownProvider = errors.New("unknown provider")

// Registry resolves a provider name to its client. It is safe for concurrent
// reads after construction.
type Registry struct {
	clients map[string]Client
}

// NewRegistry builds the default provider set over a shared HTTP client.
func NewRegistry() *Registry {
	httpc := &http.Client{Timeout: 30 * time.Second}
	return &Registry{clients: map[string]Client{
		"openai":    &openAIClient{http: httpc, baseURL: "https://api.openai.com/v1"},
		"mistral":   &openAIClient{http: httpc, baseURL: "https://api.mistral.ai/v1"},
		"anthropic": &anthropicClient{http: httpc, baseURL: "https://api.anthropic.com/v1"},
	}}
}

// Complete dispatches to the named provider's client.
func (r *Registry) Complete(ctx context.Context, provider, apiKey string, req CompletionRequest) (CompletionResult, error) {
	c, ok := r.clients[provider]
	if !ok {
		return CompletionResult{}, ErrUnknownProvider
	}
	return c.Complete(ctx, apiKey, req)
}

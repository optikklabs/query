package llmproviders

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/shared/errorcode"
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

// providers lists every provider the registry can call.
var providers = []string{"anthropic", "mistral", "openai"}

// ValidateProvider returns a validation error unless provider is supported.
func ValidateProvider(provider string) error {
	if slices.Contains(providers, provider) {
		return nil
	}
	return errorcode.ValidationError{Msg: "provider must be one of " + strings.Join(providers, ", ")}
}

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

// maxResponseBytes caps how much of a provider response is read.
const maxResponseBytes = 1 << 20

// apiError is the error object both provider APIs return.
type apiError struct {
	Message string `json:"message"`
}

// postJSON sends body as JSON and decodes the response into out, returning
// the HTTP status for the caller to interpret.
func postJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, body, out any) (int, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(out); err != nil {
		return resp.StatusCode, fmt.Errorf("invalid response (status %d)", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

// providerError describes a failed provider call, preferring the provider's
// own error message.
func providerError(provider string, status int, e *apiError) error {
	if e != nil && e.Message != "" {
		return fmt.Errorf("%s: %s (status %d)", provider, e.Message, status)
	}
	return fmt.Errorf("%s: request failed with status %d", provider, status)
}

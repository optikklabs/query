package llmproviders

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type anthropicClient struct {
	http    *http.Client
	baseURL string
}

const (
	anthropicVersion = "2023-06-01"
	// anthropicMaxTokens applies when the request sets no limit; the API
	// requires one.
	anthropicMaxTokens = 1024
)

type anthropicRequest struct {
	Model       string    `json:"model"`
	System      string    `json:"system,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *apiError `json:"error"`
}

func (c *anthropicClient) Complete(ctx context.Context, apiKey string, req CompletionRequest) (CompletionResult, error) {
	system, messages := splitSystem(req.Messages)
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = anthropicMaxTokens
	}
	var resp anthropicResponse
	status, err := postJSON(ctx, c.http, c.baseURL+"/messages",
		map[string]string{"x-api-key": apiKey, "anthropic-version": anthropicVersion},
		anthropicRequest{
			Model:       req.Model,
			System:      system,
			Messages:    messages,
			Temperature: req.Temperature,
			MaxTokens:   maxTokens,
		}, &resp)
	if err != nil {
		return CompletionResult{}, fmt.Errorf("anthropic: %w", err)
	}
	if status != http.StatusOK {
		return CompletionResult{}, providerError("anthropic", status, resp.Error)
	}
	if len(resp.Content) == 0 {
		return CompletionResult{}, errors.New("anthropic: empty completion")
	}
	return CompletionResult{
		Output:       resp.Content[0].Text,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	}, nil
}

// splitSystem moves system messages into Anthropic's separate system field,
// joining several with blank lines.
func splitSystem(in []Message) (string, []Message) {
	var system []string
	out := make([]Message, 0, len(in))
	for _, m := range in {
		if m.Role == "system" {
			system = append(system, m.Content)
			continue
		}
		out = append(out, m)
	}
	return strings.Join(system, "\n\n"), out
}

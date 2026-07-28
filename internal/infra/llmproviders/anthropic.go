package llmproviders

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type anthropicClient struct {
	http    *http.Client
	baseURL string
}

const anthropicVersion = "2023-06-01"

const anthropicMaxTokens = 1024

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
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *anthropicClient) Complete(ctx context.Context, apiKey string, req CompletionRequest) (CompletionResult, error) {
	system, messages := splitSystem(req.Messages)
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = anthropicMaxTokens
	}
	body, err := json.Marshal(anthropicRequest{
		Model:       req.Model,
		System:      system,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return CompletionResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return CompletionResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return CompletionResult{}, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var parsed anthropicResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return CompletionResult{}, fmt.Errorf("anthropic: invalid response (status %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return CompletionResult{}, providerError("anthropic", resp.StatusCode, errMessage(parsed.Error))
	}
	if len(parsed.Content) == 0 {
		return CompletionResult{}, fmt.Errorf("anthropic: empty completion")
	}
	return CompletionResult{
		Output:       parsed.Content[0].Text,
		InputTokens:  parsed.Usage.InputTokens,
		OutputTokens: parsed.Usage.OutputTokens,
	}, nil
}

func splitSystem(in []Message) (string, []Message) {
	var system string
	out := make([]Message, 0, len(in))
	for _, m := range in {
		if m.Role == "system" {
			if system != "" {
				system += "\n\n"
			}
			system += m.Content
			continue
		}
		out = append(out, m)
	}
	return system, out
}

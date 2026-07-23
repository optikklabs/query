package llmproviders

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// openAIClient calls the OpenAI-compatible /chat/completions endpoint. It also
// serves Mistral, whose API mirrors this shape, differing only in base URL.
type openAIClient struct {
	http    *http.Client
	baseURL string
}

type openAIChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *openAIClient) Complete(ctx context.Context, apiKey string, req CompletionRequest) (CompletionResult, error) {
	body, err := json.Marshal(openAIChatRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	})
	if err != nil {
		return CompletionResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return CompletionResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return CompletionResult{}, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var parsed openAIChatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return CompletionResult{}, fmt.Errorf("openai: invalid response (status %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return CompletionResult{}, providerError("openai", resp.StatusCode, errMessage(parsed.Error))
	}
	if len(parsed.Choices) == 0 {
		return CompletionResult{}, fmt.Errorf("openai: empty completion")
	}
	return CompletionResult{
		Output:       parsed.Choices[0].Message.Content,
		InputTokens:  parsed.Usage.PromptTokens,
		OutputTokens: parsed.Usage.CompletionTokens,
	}, nil
}

func errMessage(e *struct {
	Message string `json:"message"`
}) string {
	if e == nil {
		return ""
	}
	return e.Message
}

func providerError(provider string, status int, msg string) error {
	if msg != "" {
		return fmt.Errorf("%s: %s (status %d)", provider, msg, status)
	}
	return fmt.Errorf("%s: request failed with status %d", provider, status)
}

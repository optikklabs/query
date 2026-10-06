package llmproviders

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// openAIClient speaks the OpenAI chat-completions API, which Mistral also
// implements.
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
	Error *apiError `json:"error"`
}

func (c *openAIClient) Complete(ctx context.Context, apiKey string, req CompletionRequest) (CompletionResult, error) {
	var resp openAIChatResponse
	status, err := postJSON(ctx, c.http, c.baseURL+"/chat/completions",
		map[string]string{"Authorization": "Bearer " + apiKey}, openAIChatRequest(req), &resp)
	if err != nil {
		return CompletionResult{}, fmt.Errorf("openai: %w", err)
	}
	if status != http.StatusOK {
		return CompletionResult{}, providerError("openai", status, resp.Error)
	}
	if len(resp.Choices) == 0 {
		return CompletionResult{}, errors.New("openai: empty completion")
	}
	return CompletionResult{
		Output:       resp.Choices[0].Message.Content,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
	}, nil
}

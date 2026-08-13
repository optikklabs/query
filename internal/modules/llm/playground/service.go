package playground

import (
	"context"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/infra/llmproviders"
	"github.com/optikklabs/query/internal/modules/llm/pricing"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

type KeyResolver interface {
	ResolveKey(ctx context.Context, tenantID int64, provider string) (string, error)
}

type Completer interface {
	Complete(ctx context.Context, provider, apiKey string, req llmproviders.CompletionRequest) (llmproviders.CompletionResult, error)
}

type Service struct {
	keys      KeyResolver
	completer Completer
}

func NewService(keys KeyResolver, completer Completer) *Service {
	return &Service{keys: keys, completer: completer}
}

var validProvider = map[string]struct{}{"openai": {}, "anthropic": {}, "mistral": {}}

func (s *Service) Complete(ctx context.Context, tenantID int64, req CompleteRequest) (CompleteResponse, error) {
	if _, ok := validProvider[req.Provider]; !ok {
		return CompleteResponse{}, errorcode.ValidationError{Msg: "provider must be openai, anthropic or mistral"}
	}
	if strings.TrimSpace(req.Model) == "" {
		return CompleteResponse{}, errorcode.ValidationError{Msg: "model is required"}
	}
	if len(req.Messages) == 0 {
		return CompleteResponse{}, errorcode.ValidationError{Msg: "messages must not be empty"}
	}
	apiKey, err := s.keys.ResolveKey(ctx, tenantID, req.Provider)
	if err != nil {
		return CompleteResponse{}, err
	}

	start := time.Now()
	result, err := s.completer.Complete(ctx, req.Provider, apiKey, llmproviders.CompletionRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	})
	if err != nil {
		return CompleteResponse{}, err
	}
	return CompleteResponse{
		Output:       result.Output,
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
		LatencyMs:    time.Since(start).Milliseconds(),
		CostUsd:      pricing.CostOf(req.Model, uint64(result.InputTokens), uint64(result.OutputTokens)),
	}, nil
}



package playground

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/infra/llmproviders"
	"github.com/optikklabs/query/internal/modules/llm/pricing"
	"github.com/optikklabs/query/internal/modules/llm/providerkeys"
)

// KeyResolver decrypts a tenant's provider key. Implemented by providerkeys.
type KeyResolver interface {
	ResolveKey(ctx context.Context, tenantID int64, provider string) (string, error)
}

// Completer performs a single provider completion. Implemented by the registry.
type Completer interface {
	Complete(ctx context.Context, provider, apiKey string, req llmproviders.CompletionRequest) (llmproviders.CompletionResult, error)
}

// Service runs interactive completions, gated by a concurrency semaphore so a
// burst of playground calls cannot exhaust outbound sockets.
type Service struct {
	keys      KeyResolver
	completer Completer
	sem       chan struct{}
}

func NewService(keys KeyResolver, completer Completer, concurrency int) *Service {
	if concurrency <= 0 {
		concurrency = 4
	}
	return &Service{keys: keys, completer: completer, sem: make(chan struct{}, concurrency)}
}

type ErrValidation struct{ Msg string }

func (e ErrValidation) Error() string { return e.Msg }

var validProvider = map[string]struct{}{"openai": {}, "anthropic": {}, "mistral": {}}

// Complete resolves the tenant key, calls the provider, and prices the usage.
func (s *Service) Complete(ctx context.Context, tenantID int64, req CompleteRequest) (CompleteResponse, error) {
	if _, ok := validProvider[req.Provider]; !ok {
		return CompleteResponse{}, ErrValidation{Msg: "provider must be openai, anthropic or mistral"}
	}
	if strings.TrimSpace(req.Model) == "" {
		return CompleteResponse{}, ErrValidation{Msg: "model is required"}
	}
	if len(req.Messages) == 0 {
		return CompleteResponse{}, ErrValidation{Msg: "messages must not be empty"}
	}
	apiKey, err := s.keys.ResolveKey(ctx, tenantID, req.Provider)
	if err != nil {
		return CompleteResponse{}, err
	}

	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return CompleteResponse{}, ctx.Err()
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

// IsUnavailable reports whether the error is the no-encryption/no-key case, so
// the handler can map it to 503 rather than 500.
func IsUnavailable(err error) bool {
	return errors.Is(err, providerkeys.ErrNoEncryption) || errors.Is(err, providerkeys.ErrNotFound)
}

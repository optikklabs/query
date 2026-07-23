package datasets

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/infra/llmproviders"
	"github.com/optikklabs/query/internal/modules/llm/pricing"
	"github.com/optikklabs/query/internal/modules/llm/providerkeys"
)

// IsProviderUnavailable reports whether the error is a missing key/encryption
// config, so the handler can answer 503 rather than 500.
func IsProviderUnavailable(err error) bool {
	return errors.Is(err, providerkeys.ErrNoEncryption) || errors.Is(err, providerkeys.ErrNotFound)
}

// Hard caps keep synchronous experiment runs bounded — a background job runner
// is a deliberate future phase.
const (
	maxRunItems   = 50
	runBudget     = 5 * time.Minute
	exactMatchKey = "exact_match"
)

// KeyResolver decrypts a tenant's provider key. Implemented by providerkeys.
type KeyResolver interface {
	ResolveKey(ctx context.Context, tenantID int64, provider string) (string, error)
}

// Completer performs a single provider completion. Implemented by the registry.
type Completer interface {
	Complete(ctx context.Context, provider, apiKey string, req llmproviders.CompletionRequest) (llmproviders.CompletionResult, error)
}

// RunExperimentRequest configures a synchronous run over a dataset.
type RunExperimentRequest struct {
	Name         string  `json:"name"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	SystemPrompt string  `json:"systemPrompt,omitempty"`
	Temperature  float64 `json:"temperature"`
	MaxTokens    int     `json:"maxTokens"`
}

// ExperimentService orchestrates a dataset run: fan out completions, score by
// exact match, and persist per-item plus aggregate results.
type ExperimentService struct {
	repo      *Repository
	keys      KeyResolver
	completer Completer
}

func NewExperimentService(repo *Repository, keys KeyResolver, completer Completer) *ExperimentService {
	return &ExperimentService{repo: repo, keys: keys, completer: completer}
}

// Run executes the experiment synchronously and returns the completed run.
func (s *ExperimentService) Run(ctx context.Context, tenantID, datasetID int64, req RunExperimentRequest) (RunDetail, error) {
	if _, ok := map[string]struct{}{"openai": {}, "anthropic": {}, "mistral": {}}[req.Provider]; !ok {
		return RunDetail{}, ErrValidation{Msg: "provider must be openai, anthropic or mistral"}
	}
	if strings.TrimSpace(req.Model) == "" {
		return RunDetail{}, ErrValidation{Msg: "model is required"}
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = req.Model + " run"
	}
	ok, err := s.repo.DatasetExists(ctx, tenantID, datasetID)
	if err != nil {
		return RunDetail{}, err
	}
	if !ok {
		return RunDetail{}, ErrNotFound
	}
	items, err := s.repo.ListItems(ctx, datasetID)
	if err != nil {
		return RunDetail{}, err
	}
	if len(items) == 0 {
		return RunDetail{}, ErrValidation{Msg: "dataset has no items to run"}
	}
	if len(items) > maxRunItems {
		items = items[:maxRunItems]
	}
	apiKey, err := s.keys.ResolveKey(ctx, tenantID, req.Provider)
	if err != nil {
		return RunDetail{}, err
	}

	params, _ := json.Marshal(map[string]any{"temperature": req.Temperature, "maxTokens": req.MaxTokens})
	runID, err := s.repo.CreateRun(ctx, RunInsert{
		DatasetID: datasetID, TenantID: tenantID, Name: name,
		Provider: req.Provider, Model: req.Model, ParamsJSON: params, ItemCount: len(items),
	})
	if err != nil {
		return RunDetail{}, err
	}

	runCtx, cancel := context.WithTimeout(ctx, runBudget)
	defer cancel()
	s.execute(runCtx, tenantID, runID, req, apiKey, items)
	return s.getRun(ctx, tenantID, runID)
}

// execute runs each item, persists its result, and finalizes the aggregate.
func (s *ExperimentService) execute(ctx context.Context, tenantID, runID int64, req RunExperimentRequest, apiKey string, items []itemRow) {
	var totalCost, totalLatency, totalScore float64
	var scored, failed int

	for _, item := range items {
		messages := buildMessages(req.SystemPrompt, item.InputJSON)
		start := time.Now()
		result, err := s.completer.Complete(ctx, req.Provider, apiKey, llmproviders.CompletionRequest{
			Model: req.Model, Messages: messages, Temperature: req.Temperature, MaxTokens: req.MaxTokens,
		})
		latency := int(time.Since(start).Milliseconds())
		ri := RunItemInsert{RunID: runID, TenantID: tenantID, DatasetItemID: item.ID, LatencyMs: latency}
		if err != nil {
			ri.Error = sql.NullString{Valid: true, String: truncate(err.Error(), 1000)}
			ri.ScoresJSON = []byte("{}")
			_ = s.repo.InsertRunItem(ctx, ri)
			failed++
			continue
		}
		cost := pricing.CostOf(req.Model, uint64(result.InputTokens), uint64(result.OutputTokens))
		out, _ := json.Marshal(map[string]any{"output": result.Output,
			"inputTokens": result.InputTokens, "outputTokens": result.OutputTokens})
		ri.OutputJSON = out
		ri.CostUsd = cost
		scores := map[string]float64{}
		if score, has := exactMatch(item.ExpectedOutputJSON, result.Output); has {
			scores[exactMatchKey] = score
			totalScore += score
			scored++
		}
		ri.ScoresJSON = mustJSON(scores)
		_ = s.repo.InsertRunItem(ctx, ri)
		totalCost += cost
		totalLatency += float64(latency)
	}

	final := RunFinal{Status: "completed", TotalCostUsd: totalCost}
	if n := len(items); n > 0 {
		final.AvgLatencyMs = totalLatency / float64(n)
	}
	avgScores := map[string]float64{}
	if scored > 0 {
		avgScores[exactMatchKey] = totalScore / float64(scored)
	}
	final.AvgScoresJSON = mustJSON(avgScores)
	if failed == len(items) {
		final.Status = "failed"
		final.Error = sql.NullString{Valid: true, String: "all items failed"}
	}
	_ = s.repo.FinalizeRun(ctx, runID, final)
}

func (s *ExperimentService) getRun(ctx context.Context, tenantID, runID int64) (RunDetail, error) {
	run, err := s.repo.GetRun(ctx, tenantID, runID)
	if err != nil {
		return RunDetail{}, err
	}
	rawItems, err := s.repo.ListRunItems(ctx, runID)
	if err != nil {
		return RunDetail{}, err
	}
	detail := RunDetail{RunSummary: toRunSummary(run)}
	for _, it := range rawItems {
		detail.Items = append(detail.Items, toRunItem(it))
	}
	return detail, nil
}

// buildMessages derives the chat turns for an item. The item input may be a
// {"messages":[...]} object, an {"input":"..."} object, or a bare JSON string.
func buildMessages(systemPrompt string, inputJSON []byte) []llmproviders.Message {
	var msgs []llmproviders.Message
	if s := strings.TrimSpace(systemPrompt); s != "" {
		msgs = append(msgs, llmproviders.Message{Role: "system", Content: s})
	}
	var withMessages struct {
		Messages []llmproviders.Message `json:"messages"`
		Input    string                 `json:"input"`
	}
	if err := json.Unmarshal(inputJSON, &withMessages); err == nil {
		if len(withMessages.Messages) > 0 {
			return append(msgs, withMessages.Messages...)
		}
		if withMessages.Input != "" {
			return append(msgs, llmproviders.Message{Role: "user", Content: withMessages.Input})
		}
	}
	return append(msgs, llmproviders.Message{Role: "user", Content: string(inputJSON)})
}

// exactMatch scores 1.0 when the trimmed output equals the expected text. It
// reports has=false when no expected output is present, so unscored items don't
// drag the average down.
func exactMatch(expectedJSON []byte, output string) (score float64, has bool) {
	expected := strings.TrimSpace(extractExpected(expectedJSON))
	if expected == "" {
		return 0, false
	}
	if strings.TrimSpace(output) == expected {
		return 1, true
	}
	return 0, true
}

func extractExpected(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj struct {
		Output string `json:"output"`
		Text   string `json:"text"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		if obj.Output != "" {
			return obj.Output
		}
		return obj.Text
	}
	return string(raw)
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

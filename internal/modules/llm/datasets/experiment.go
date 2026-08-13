package datasets

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/infra/llmproviders"
	"github.com/optikklabs/query/internal/modules/llm/pricing"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"golang.org/x/sync/errgroup"
)



const (
	maxRunItems   = 50
	runBudget     = 5 * time.Minute
	itemWorkers   = 4
	exactMatchKey = "exact_match"
)

type KeyResolver interface {
	ResolveKey(ctx context.Context, tenantID int64, provider string) (string, error)
}

type Completer interface {
	Complete(ctx context.Context, provider, apiKey string, req llmproviders.CompletionRequest) (llmproviders.CompletionResult, error)
}

type RunExperimentRequest struct {
	Name         string  `json:"name"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	SystemPrompt string  `json:"systemPrompt,omitempty"`
	Temperature  float64 `json:"temperature"`
	MaxTokens    int     `json:"maxTokens"`
}

type ExperimentService struct {
	repo      *Repository
	keys      KeyResolver
	completer Completer
	ctx       context.Context
	cancel    context.CancelFunc
	jobs      errgroup.Group
}

func NewExperimentService(repo *Repository, keys KeyResolver, completer Completer) *ExperimentService {
	ctx, cancel := context.WithCancel(context.Background())
	return &ExperimentService{
		repo: repo, keys: keys, completer: completer,
		ctx: ctx, cancel: cancel,
	}
}

type experimentJob struct {
	tenantID int64
	runID    int64
	req      RunExperimentRequest
	apiKey   string
	items    []itemRow
}

func (s *ExperimentService) Stop() error {
	s.cancel()
	return s.jobs.Wait()
}

func (s *ExperimentService) executeJob(job experimentJob) {
	ctx, cancel := context.WithTimeout(s.ctx, runBudget)
	err := s.execute(ctx, job)
	cancel()
	if err != nil {
		slog.Error("llm experiment failed", slog.Int64("run_id", job.runID), slog.Any("error", err))
		failCtx, failCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = s.repo.FinalizeRun(failCtx, job.runID, RunFinal{
			Status: "failed", Error: sql.NullString{Valid: true, String: truncate(err.Error(), 1000)},
		})
		failCancel()
	}
}

func (s *ExperimentService) Run(ctx context.Context, tenantID, datasetID int64, req RunExperimentRequest) (RunDetail, error) {
	if _, ok := map[string]struct{}{"openai": {}, "anthropic": {}, "mistral": {}}[req.Provider]; !ok {
		return RunDetail{}, errorcode.ValidationError{Msg: "provider must be openai, anthropic or mistral"}
	}
	if strings.TrimSpace(req.Model) == "" {
		return RunDetail{}, errorcode.ValidationError{Msg: "model is required"}
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
		return RunDetail{}, errorcode.ValidationError{Msg: "dataset has no items to run"}
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

	job := experimentJob{tenantID: tenantID, runID: runID, req: req, apiKey: apiKey, items: items}
	s.jobs.Go(func() error {
		s.executeJob(job)
		return nil
	})
	return s.getRun(ctx, tenantID, runID)
}

type completedItem struct {
	row    RunItemInsert
	cost   float64
	score  float64
	scored bool
	failed bool
}

func (s *ExperimentService) execute(ctx context.Context, job experimentJob) error {
	results := make([]completedItem, len(job.items))
	g, groupCtx := errgroup.WithContext(ctx)
	g.SetLimit(itemWorkers)
	for i, item := range job.items {
		g.Go(func() error {
			results[i] = s.completeItem(groupCtx, job, item)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	var totalCost, totalLatency, totalScore float64
	var scored, failed int
	for _, result := range results {
		if err := s.repo.InsertRunItem(ctx, result.row); err != nil {
			return fmt.Errorf("insert run item: %w", err)
		}
		if result.failed {
			failed++
			continue
		}
		totalCost += result.cost
		totalLatency += float64(result.row.LatencyMs)
		if result.scored {
			totalScore += result.score
			scored++
		}
	}

	final := RunFinal{Status: "completed", TotalCostUsd: totalCost}
	if n := len(job.items); n > 0 {
		final.AvgLatencyMs = totalLatency / float64(n)
	}
	avgScores := map[string]float64{}
	if scored > 0 {
		avgScores[exactMatchKey] = totalScore / float64(scored)
	}
	final.AvgScoresJSON = mustJSON(avgScores)
	if failed == len(job.items) {
		final.Status = "failed"
		final.Error = sql.NullString{Valid: true, String: "all items failed"}
	}
	return s.repo.FinalizeRun(ctx, job.runID, final)
}

func (s *ExperimentService) completeItem(ctx context.Context, job experimentJob, item itemRow) completedItem {
	start := time.Now()
	result, err := s.completer.Complete(ctx, job.req.Provider, job.apiKey, llmproviders.CompletionRequest{
		Model: job.req.Model, Messages: buildMessages(job.req.SystemPrompt, item.InputJSON),
		Temperature: job.req.Temperature, MaxTokens: job.req.MaxTokens,
	})
	row := RunItemInsert{
		RunID: job.runID, TenantID: job.tenantID, DatasetItemID: item.ID,
		LatencyMs: int(time.Since(start).Milliseconds()), ScoresJSON: []byte("{}"),
	}
	if err != nil {
		row.Error = sql.NullString{Valid: true, String: truncate(err.Error(), 1000)}
		return completedItem{row: row, failed: true}
	}
	cost := pricing.CostOf(job.req.Model, uint64(result.InputTokens), uint64(result.OutputTokens))
	row.OutputJSON, _ = json.Marshal(map[string]any{"output": result.Output,
		"inputTokens": result.InputTokens, "outputTokens": result.OutputTokens})
	row.CostUsd = cost
	score, scored := exactMatch(item.ExpectedOutputJSON, result.Output)
	if scored {
		row.ScoresJSON = mustJSON(map[string]float64{exactMatchKey: score})
	}
	return completedItem{row: row, cost: cost, score: score, scored: scored}
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

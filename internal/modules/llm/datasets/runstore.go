package datasets

import (
	"context"
	"database/sql"
	"time"

	dbutil "github.com/optikklabs/query/internal/infra/database"
)

type RunInsert struct {
	DatasetID       int64
	TenantID        int64
	Name            string
	Provider        string
	Model           string
	PromptVersionID sql.NullInt64
	ParamsJSON      []byte
	ItemCount       int
}

type RunItemInsert struct {
	RunID         int64
	TenantID      int64
	DatasetItemID int64
	OutputJSON    []byte
	LatencyMs     int
	CostUsd       float64
	ScoresJSON    []byte
	Error         sql.NullString
}

type RunFinal struct {
	Status        string
	AvgScoresJSON []byte
	TotalCostUsd  float64
	AvgLatencyMs  float64
	Error         sql.NullString
}

func (r *Repository) CreateRun(ctx context.Context, in RunInsert) (int64, error) {
	res, err := dbutil.ExecSQL(ctx, r.db, "datasets.CreateRun", `
		INSERT INTO optikk.llm_experiment_runs
		  (dataset_id, tenant_id, name, provider, model, prompt_version_id, params_json,
		   status, item_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'running', ?, ?)`,
		in.DatasetID, in.TenantID, in.Name, in.Provider, in.Model, in.PromptVersionID,
		rawOrDefault(in.ParamsJSON, "{}"), in.ItemCount, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repository) InsertRunItem(ctx context.Context, in RunItemInsert) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "datasets.InsertRunItem", `
		INSERT INTO optikk.llm_experiment_run_items
		  (run_id, tenant_id, dataset_item_id, output_json, latency_ms, cost_usd, scores_json, error, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.RunID, in.TenantID, in.DatasetItemID, nullableRaw(in.OutputJSON),
		in.LatencyMs, in.CostUsd, rawOrDefault(in.ScoresJSON, "{}"), in.Error, time.Now().UTC())
	return err
}

func (r *Repository) FinalizeRun(ctx context.Context, runID int64, f RunFinal) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "datasets.FinalizeRun", `
		UPDATE optikk.llm_experiment_runs
		   SET status = ?, avg_scores_json = ?, total_cost_usd = ?, avg_latency_ms = ?,
		       error = ?, completed_at = ?
		 WHERE id = ?`,
		f.Status, rawOrDefault(f.AvgScoresJSON, "{}"), f.TotalCostUsd, f.AvgLatencyMs,
		f.Error, time.Now().UTC(), runID)
	return err
}

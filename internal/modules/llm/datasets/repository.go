package datasets

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
	dbutil "github.com/optikklabs/query/internal/infra/database"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql")}
}

const datasetCols = `
  d.id, d.name, d.description, d.updated_at, d.created_at,
  (SELECT COUNT(*) FROM optikk.llm_dataset_items i WHERE i.dataset_id = d.id) AS item_count,
  (SELECT COUNT(*) FROM optikk.llm_experiment_runs r WHERE r.dataset_id = d.id) AS run_count
`

func (r *Repository) List(ctx context.Context, tenantID int64) ([]datasetRow, error) {
	var rows []datasetRow
	err := dbutil.SelectSQL(ctx, r.db, "datasets.List", &rows,
		`SELECT `+datasetCols+`
		   FROM optikk.llm_datasets d
		  WHERE d.tenant_id = ?
		  ORDER BY COALESCE(d.updated_at, d.created_at) DESC, d.id DESC`, tenantID)
	return rows, err
}

func (r *Repository) Get(ctx context.Context, tenantID, id int64) (datasetRow, error) {
	var row datasetRow
	err := dbutil.GetSQL(ctx, r.db, "datasets.Get", &row,
		`SELECT `+datasetCols+`
		   FROM optikk.llm_datasets d
		  WHERE d.tenant_id = ? AND d.id = ? LIMIT 1`, tenantID, id)
	return row, err
}

func (r *Repository) Create(ctx context.Context, tenantID, userID int64, name string, desc sql.NullString) (int64, error) {
	var createdBy sql.NullInt64
	if userID > 0 {
		createdBy = sql.NullInt64{Valid: true, Int64: userID}
	}
	res, err := dbutil.ExecSQL(ctx, r.db, "datasets.Create",
		`INSERT INTO optikk.llm_datasets (tenant_id, name, description, created_at, created_by_user_id)
		 VALUES (?, ?, ?, ?, ?)`,
		tenantID, name, desc, time.Now().UTC(), createdBy)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repository) Delete(ctx context.Context, tenantID, id int64) error {
	res, err := dbutil.ExecSQL(ctx, r.db, "datasets.Delete",
		`DELETE FROM optikk.llm_datasets WHERE tenant_id = ? AND id = ?`, tenantID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) AddItems(ctx context.Context, tenantID, datasetID int64, items []ItemInput) (int, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	for _, it := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO optikk.llm_dataset_items
			  (dataset_id, tenant_id, input_json, expected_output_json, metadata_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			datasetID, tenantID, rawOrEmptyObject(it.Input),
			nullableRaw(it.ExpectedOutput), rawOrEmptyObject(it.Metadata), now); err != nil {
			return 0, err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE optikk.llm_datasets SET updated_at = ? WHERE id = ?`, now, datasetID); err != nil {
		return 0, err
	}
	return len(items), tx.Commit()
}

func (r *Repository) ListItems(ctx context.Context, datasetID int64) ([]itemRow, error) {
	var rows []itemRow
	err := dbutil.SelectSQL(ctx, r.db, "datasets.ListItems", &rows,
		`SELECT id, input_json, expected_output_json, metadata_json, created_at
		   FROM optikk.llm_dataset_items WHERE dataset_id = ? ORDER BY id ASC`, datasetID)
	return rows, err
}

func (r *Repository) ListRuns(ctx context.Context, datasetID int64) ([]runRow, error) {
	var rows []runRow
	err := dbutil.SelectSQL(ctx, r.db, "datasets.ListRuns", &rows,
		`SELECT id, name, provider, model, status, item_count, avg_scores_json,
		        total_cost_usd, avg_latency_ms, error, created_at, completed_at
		   FROM optikk.llm_experiment_runs WHERE dataset_id = ? ORDER BY created_at DESC LIMIT 50`, datasetID)
	return rows, err
}

func (r *Repository) GetRun(ctx context.Context, tenantID, runID int64) (runRow, error) {
	var row runRow
	err := dbutil.GetSQL(ctx, r.db, "datasets.GetRun", &row,
		`SELECT id, name, provider, model, status, item_count, avg_scores_json,
		        total_cost_usd, avg_latency_ms, error, created_at, completed_at
		   FROM optikk.llm_experiment_runs WHERE tenant_id = ? AND id = ? LIMIT 1`, tenantID, runID)
	return row, err
}

func (r *Repository) ListRunItems(ctx context.Context, runID int64) ([]runItemRow, error) {
	var rows []runItemRow
	err := dbutil.SelectSQL(ctx, r.db, "datasets.ListRunItems", &rows,
		`SELECT dataset_item_id, output_json, latency_ms, cost_usd, scores_json, error
		   FROM optikk.llm_experiment_run_items WHERE run_id = ? ORDER BY id ASC`, runID)
	return rows, err
}

func (r *Repository) DatasetExists(ctx context.Context, tenantID, id int64) (bool, error) {
	var n int
	err := dbutil.GetSQL(ctx, r.db, "datasets.Exists", &n,
		`SELECT COUNT(*) FROM optikk.llm_datasets WHERE tenant_id = ? AND id = ?`, tenantID, id)
	return n > 0, err
}

func rawOrEmptyObject(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	return raw
}

func nullableRaw(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}

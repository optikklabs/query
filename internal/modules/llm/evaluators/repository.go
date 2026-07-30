package evaluators

import (
	"context"
	"database/sql"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/jmoiron/sqlx"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/shared/chargs"
)

type Repository struct {
	db *sqlx.DB
	ch clickhouse.Conn
}

func NewRepository(db *sql.DB, ch clickhouse.Conn) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql"), ch: ch}
}

const evaluatorCols = `
  id, name, score_name, judge_model, target, sampling_pct, data_type,
  categories_json, prompt_template, enabled, created_at, updated_at
`

func (r *Repository) List(ctx context.Context, tenantID int64) ([]evaluatorRow, error) {
	var rows []evaluatorRow
	err := dbutil.SelectSQL(ctx, r.db, "evaluators.List", &rows,
		`SELECT `+evaluatorCols+` FROM optikk.llm_evaluators
		  WHERE tenant_id = ? ORDER BY name ASC`, tenantID)
	return rows, err
}

func (r *Repository) Get(ctx context.Context, tenantID, id int64) (evaluatorRow, error) {
	var row evaluatorRow
	err := dbutil.GetSQL(ctx, r.db, "evaluators.Get", &row,
		`SELECT `+evaluatorCols+` FROM optikk.llm_evaluators
		  WHERE tenant_id = ? AND id = ? LIMIT 1`, tenantID, id)
	return row, err
}

type insertArgs struct {
	TenantID       int64
	Name           string
	ScoreName      string
	JudgeModel     sql.NullString
	Target         string
	SamplingPct    int
	DataType       string
	CategoriesJSON []byte
	PromptTemplate sql.NullString
	Enabled        bool
	CreatedBy      sql.NullInt64
}

func (r *Repository) Create(ctx context.Context, a insertArgs) (int64, error) {
	res, err := dbutil.ExecSQL(ctx, r.db, "evaluators.Create", `
		INSERT INTO optikk.llm_evaluators
		  (tenant_id, name, score_name, judge_model, target, sampling_pct, data_type,
		   categories_json, prompt_template, enabled, created_at, created_by_user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.TenantID, a.Name, a.ScoreName, a.JudgeModel, a.Target, a.SamplingPct, a.DataType,
		a.CategoriesJSON, a.PromptTemplate, a.Enabled, time.Now().UTC(), a.CreatedBy)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repository) Update(ctx context.Context, tenantID, id int64, a insertArgs) error {
	res, err := dbutil.ExecSQL(ctx, r.db, "evaluators.Update", `
		UPDATE optikk.llm_evaluators
		   SET name = ?, score_name = ?, judge_model = ?, target = ?, sampling_pct = ?,
		       data_type = ?, categories_json = ?, prompt_template = ?, enabled = ?, updated_at = ?
		 WHERE tenant_id = ? AND id = ?`,
		a.Name, a.ScoreName, a.JudgeModel, a.Target, a.SamplingPct, a.DataType,
		a.CategoriesJSON, a.PromptTemplate, a.Enabled, time.Now().UTC(), tenantID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, tenantID, id int64) error {
	res, err := dbutil.ExecSQL(ctx, r.db, "evaluators.Delete",
		`DELETE FROM optikk.llm_evaluators WHERE tenant_id = ? AND id = ?`, tenantID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type scoreAgg struct {
	Name  string  `ch:"name"`
	Count int64   `ch:"cnt"`
	Mean  float64 `ch:"mean"`
}

func (r *Repository) ScoreAggregates(ctx context.Context, tenantID, startMs, endMs int64, names []string) (map[string]scoreAgg, error) {
	out := map[string]scoreAgg{}
	if len(names) == 0 {
		return out, nil
	}
	query := `
		SELECT name, count() AS cnt, avg(value) AS mean
		FROM optikk.llm_scores
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE name IN @names
		GROUP BY name`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), clickhouse.Named("names", names))
	var rows []scoreAgg
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.ch, "evaluators.ScoreAggregates", &rows, query, args...); err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.Name] = row
	}
	return out, nil
}

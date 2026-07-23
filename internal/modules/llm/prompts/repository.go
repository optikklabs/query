package prompts

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
	dbutil "github.com/optikklabs/query/internal/infra/database"
)

// Repository owns MySQL persistence for prompts and their versions.
type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql")}
}

type promptInsertArgs struct {
	TenantID    int64
	Name        string
	Type        string
	Description sql.NullString
	TagsJSON    []byte
	CreatedBy   sql.NullInt64
}

type versionInsertArgs struct {
	PromptID      int64
	TenantID      int64
	TemplateJSON  []byte
	VariablesJSON []byte
	Notes         sql.NullString
	Production    bool
	CreatedBy     sql.NullInt64
}

// CreatePrompt inserts the prompt and its first version atomically. The first
// version is always v1 and starts in production so the prompt is usable.
func (r *Repository) CreatePrompt(ctx context.Context, p promptInsertArgs, v versionInsertArgs) (int64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO optikk.llm_prompts
		  (tenant_id, name, type, description, tags_json, created_at, created_by_user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.TenantID, p.Name, p.Type, p.Description, p.TagsJSON, time.Now().UTC(), p.CreatedBy)
	if err != nil {
		return 0, err
	}
	promptID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	status := "production"
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO optikk.llm_prompt_versions
		  (prompt_id, tenant_id, version, template_json, variables_json, notes, status, created_at, created_by_user_id)
		VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?)`,
		promptID, v.TenantID, v.TemplateJSON, v.VariablesJSON, v.Notes, status, time.Now().UTC(), v.CreatedBy); err != nil {
		return 0, err
	}
	return promptID, tx.Commit()
}

// CreateVersion appends the next sequential version. When production is set it
// demotes the current production version in the same transaction.
func (r *Repository) CreateVersion(ctx context.Context, v versionInsertArgs) (int, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var maxVersion int
	if err := tx.GetContext(ctx, &maxVersion,
		`SELECT COALESCE(MAX(version), 0) FROM optikk.llm_prompt_versions WHERE prompt_id = ?`,
		v.PromptID); err != nil {
		return 0, err
	}
	next := maxVersion + 1
	status := "draft"
	if v.Production {
		status = "production"
		if err := demoteProduction(ctx, tx, v.PromptID); err != nil {
			return 0, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO optikk.llm_prompt_versions
		  (prompt_id, tenant_id, version, template_json, variables_json, notes, status, created_at, created_by_user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.PromptID, v.TenantID, next, v.TemplateJSON, v.VariablesJSON, v.Notes, status, time.Now().UTC(), v.CreatedBy); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE optikk.llm_prompts SET updated_at = ? WHERE id = ?`, time.Now().UTC(), v.PromptID); err != nil {
		return 0, err
	}
	return next, tx.Commit()
}

// SetVersionStatus flips a version's lifecycle. Promoting to production demotes
// the previous production version so exactly one stays live.
func (r *Repository) SetVersionStatus(ctx context.Context, promptID int64, version int, status string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if status == "production" {
		if err := demoteProduction(ctx, tx, promptID); err != nil {
			return err
		}
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE optikk.llm_prompt_versions SET status = ? WHERE prompt_id = ? AND version = ?`,
		status, promptID, version)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE optikk.llm_prompts SET updated_at = ? WHERE id = ?`, time.Now().UTC(), promptID); err != nil {
		return err
	}
	return tx.Commit()
}

// demoteProduction archives any current production version of the prompt.
func demoteProduction(ctx context.Context, tx *sqlx.Tx, promptID int64) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE optikk.llm_prompt_versions SET status = 'archived' WHERE prompt_id = ? AND status = 'production'`,
		promptID)
	return err
}

func (r *Repository) GetPromptByName(ctx context.Context, tenantID int64, name string) (promptRow, error) {
	var row promptRow
	err := dbutil.GetSQL(ctx, r.db, "prompts.GetByName", &row, `
		SELECT id, name, type, description, tags_json, updated_at, created_at
		  FROM optikk.llm_prompts WHERE tenant_id = ? AND name = ? LIMIT 1`, tenantID, name)
	return row, err
}

func (r *Repository) ListVersions(ctx context.Context, promptID int64) ([]versionRow, error) {
	var rows []versionRow
	err := dbutil.SelectSQL(ctx, r.db, "prompts.ListVersions", &rows, `
		SELECT version, template_json, variables_json, notes, status, created_at
		  FROM optikk.llm_prompt_versions WHERE prompt_id = ? ORDER BY version DESC`, promptID)
	return rows, err
}

// promptCatalogRow augments a prompt with aggregate version info for the list.
type promptCatalogRow struct {
	promptRow
	VersionCount      int  `db:"version_count"`
	ProductionVersion *int `db:"production_version"`
}

func (r *Repository) ListPrompts(ctx context.Context, tenantID int64) ([]promptCatalogRow, error) {
	var rows []promptCatalogRow
	err := dbutil.SelectSQL(ctx, r.db, "prompts.List", &rows, `
		SELECT p.id, p.name, p.type, p.description, p.tags_json, p.updated_at, p.created_at,
		       (SELECT COUNT(*) FROM optikk.llm_prompt_versions v WHERE v.prompt_id = p.id) AS version_count,
		       (SELECT v.version FROM optikk.llm_prompt_versions v
		         WHERE v.prompt_id = p.id AND v.status = 'production' LIMIT 1) AS production_version
		  FROM optikk.llm_prompts p
		 WHERE p.tenant_id = ?
		 ORDER BY COALESCE(p.updated_at, p.created_at) DESC`, tenantID)
	return rows, err
}

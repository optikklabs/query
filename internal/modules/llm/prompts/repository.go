package prompts

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
	version, err := tx.ExecContext(ctx, `
		INSERT INTO optikk.llm_prompt_versions
		  (prompt_id, tenant_id, version, template_json, variables_json, notes, status, created_at, created_by_user_id)
		VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?)`,
		promptID, v.TenantID, v.TemplateJSON, v.VariablesJSON, v.Notes, "draft", time.Now().UTC(), v.CreatedBy)
	if err != nil {
		return 0, err
	}
	versionID, err := version.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE optikk.llm_prompts SET production_version_id = ? WHERE id = ?`, versionID, promptID); err != nil {
		return 0, err
	}
	return promptID, tx.Commit()
}

func (r *Repository) CreateVersion(ctx context.Context, v versionInsertArgs) (int, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var promptID int64
	if err := tx.GetContext(ctx, &promptID,
		`SELECT id FROM optikk.llm_prompts WHERE id = ? AND tenant_id = ? FOR UPDATE`,
		v.PromptID, v.TenantID); err != nil {
		return 0, err
	}
	var maxVersion int
	if err := tx.GetContext(ctx, &maxVersion,
		`SELECT COALESCE(MAX(version), 0) FROM optikk.llm_prompt_versions WHERE prompt_id = ?`,
		promptID); err != nil {
		return 0, err
	}
	next := maxVersion + 1
	res, err := tx.ExecContext(ctx, `
		INSERT INTO optikk.llm_prompt_versions
		  (prompt_id, tenant_id, version, template_json, variables_json, notes, status, created_at, created_by_user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		promptID, v.TenantID, next, v.TemplateJSON, v.VariablesJSON, v.Notes, "draft", time.Now().UTC(), v.CreatedBy)
	if err != nil {
		return 0, err
	}
	versionID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE optikk.llm_prompts
		   SET production_version_id = IF(?, ?, production_version_id), updated_at = ?
		 WHERE id = ?`, v.Production, versionID, time.Now().UTC(), promptID); err != nil {
		return 0, err
	}
	return next, tx.Commit()
}

func (r *Repository) SetVersionStatus(ctx context.Context, promptID int64, version int, status string) error {
	res, err := dbutil.ExecSQL(ctx, r.db, "prompts.SetVersionStatus", `
		UPDATE optikk.llm_prompts p
		JOIN optikk.llm_prompt_versions v ON v.prompt_id = p.id AND v.version = ?
		   SET v.status = IF(? = 'production', 'draft', ?),
		       p.production_version_id = CASE
		           WHEN ? = 'production' THEN v.id
		           WHEN p.production_version_id = v.id THEN NULL
		           ELSE p.production_version_id END,
		       p.updated_at = ?
		 WHERE p.id = ?`, version, status, status, status, time.Now().UTC(), promptID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	var exists bool
	err = dbutil.GetSQL(ctx, r.db, "prompts.VersionExists", &exists,
		`SELECT EXISTS(SELECT 1 FROM optikk.llm_prompt_versions WHERE prompt_id = ? AND version = ?)`,
		promptID, version)
	if err == nil && !exists {
		return sql.ErrNoRows
	}
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
		SELECT v.version, v.template_json, v.variables_json, v.notes,
		       IF(v.id = p.production_version_id, 'production',
		          IF(v.status = 'production', 'draft', v.status)) AS status, v.created_at
		  FROM optikk.llm_prompt_versions v
		  JOIN optikk.llm_prompts p ON p.id = v.prompt_id
		 WHERE v.prompt_id = ? ORDER BY v.version DESC`, promptID)
	return rows, err
}

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
		       pv.version AS production_version
		  FROM optikk.llm_prompts p
		  LEFT JOIN optikk.llm_prompt_versions pv ON pv.id = p.production_version_id
		 WHERE p.tenant_id = ?
		 ORDER BY COALESCE(p.updated_at, p.created_at) DESC, p.id DESC`, tenantID)
	return rows, err
}

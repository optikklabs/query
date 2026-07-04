package dashboards

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

// pageInsertArgs bundles the column values for page INSERT/UPDATE.
type pageInsertArgs struct {
	TenantID        int64
	Name            string
	Description     sql.NullString
	Icon            string
	IconColor       string
	TagsJSON        []byte
	IsFavorite      bool
	CreatedByUserID sql.NullInt64
}

const insertPage = `
INSERT INTO optikk.dashboard_pages
  (tenant_id, name, description, icon, icon_color, tags_json, is_favorite,
   created_by_user_id, created_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?)
`

func (r *Repository) CreatePage(ctx context.Context, row pageInsertArgs) (int64, error) {
	res, err := dbutil.ExecSQL(ctx, r.db, "dashboards.CreatePage", insertPage,
		row.TenantID, row.Name, row.Description, row.Icon, row.IconColor,
		row.TagsJSON, row.IsFavorite, row.CreatedByUserID, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

const updatePage = `
UPDATE optikk.dashboard_pages
   SET name = ?, description = ?, icon = ?, icon_color = ?,
       tags_json = ?, is_favorite = ?, updated_at = ?
 WHERE id = ? AND tenant_id = ?
`

func (r *Repository) UpdatePage(ctx context.Context, id, tenantID int64, row pageInsertArgs) error {
	res, err := dbutil.ExecSQL(ctx, r.db, "dashboards.UpdatePage", updatePage,
		row.Name, row.Description, row.Icon, row.IconColor,
		row.TagsJSON, row.IsFavorite, time.Now().UTC(), id, tenantID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) DeletePage(ctx context.Context, id, tenantID int64) error {
	res, err := dbutil.ExecSQL(ctx, r.db, "dashboards.DeletePage",
		`DELETE FROM optikk.dashboard_pages WHERE id = ? AND tenant_id = ?`, id, tenantID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// selectPageCols resolves widget_count via a scoped subquery and the owner name
// via a left join, so the catalog never needs a GROUP BY.
const selectPageCols = `
  p.id, p.tenant_id, p.name, p.description, p.icon, p.icon_color,
  p.tags_json, p.is_favorite, p.created_by_user_id, p.created_at, p.updated_at,
  (SELECT COUNT(*) FROM optikk.dashboards d WHERE d.page_id = p.id) AS widget_count,
  u.name AS owner_name
  FROM optikk.dashboard_pages p
  LEFT JOIN optikk.users u ON u.id = p.created_by_user_id
`

func (r *Repository) GetPageByID(ctx context.Context, id, tenantID int64) (DashboardPageRow, error) {
	var row DashboardPageRow
	q := fmt.Sprintf("SELECT %s WHERE p.id = ? AND p.tenant_id = ? LIMIT 1", selectPageCols)
	if err := dbutil.GetSQL(ctx, r.db, "dashboards.GetPageByID", &row, q, id, tenantID); err != nil {
		return row, err
	}
	return row, nil
}

func (r *Repository) ListPages(ctx context.Context, tenantID int64, q ListPagesQuery) ([]DashboardPageRow, int, error) {
	where, args := pageFilters(tenantID, q)
	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := "SELECT COUNT(*) FROM optikk.dashboard_pages p WHERE " + whereSQL
	if err := dbutil.GetSQL(ctx, r.db, "dashboards.CountPages", &total, countSQL, args...); err != nil {
		return nil, 0, err
	}

	limit := q.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}
	listArgs := append(append([]any{}, args...), limit, offset)
	listSQL := fmt.Sprintf(`SELECT %s WHERE %s
		ORDER BY p.is_favorite DESC, p.updated_at DESC, p.created_at DESC
		LIMIT ? OFFSET ?`, selectPageCols, whereSQL)

	var rows []DashboardPageRow
	if err := dbutil.SelectSQL(ctx, r.db, "dashboards.ListPages", &rows, listSQL, listArgs...); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// pageFilters builds the shared WHERE clause for list and count.
func pageFilters(tenantID int64, q ListPagesQuery) ([]string, []any) {
	where := []string{"p.tenant_id = ?"}
	args := []any{tenantID}
	if q.Search != "" {
		where = append(where, "p.name LIKE ?")
		args = append(args, "%"+q.Search+"%")
	}
	if q.Favorite {
		where = append(where, "p.is_favorite = 1")
	}
	if q.Tag != "" {
		where = append(where, "JSON_CONTAINS(p.tags_json, JSON_QUOTE(?))")
		args = append(args, q.Tag)
	}
	return where, args
}

// widgetInsertArgs bundles the column values for widget INSERT/UPDATE.
type widgetInsertArgs struct {
	PageID        int64
	TenantID      int64
	Title         sql.NullString
	PanelType     string
	LayoutVariant sql.NullString
	SpecJSON      []byte
	LayoutJSON    []byte
	Position      int
}

const selectWidgetCols = `
  id, page_id, tenant_id, title, panel_type, layout_variant,
  spec_json, layout_json, position, created_at, updated_at
`

func (r *Repository) ListWidgets(ctx context.Context, pageID, tenantID int64) ([]DashboardRow, error) {
	q := fmt.Sprintf(`SELECT %s FROM optikk.dashboards
		WHERE page_id = ? AND tenant_id = ?
		ORDER BY position ASC, id ASC`, selectWidgetCols)
	var rows []DashboardRow
	if err := dbutil.SelectSQL(ctx, r.db, "dashboards.ListWidgets", &rows, q, pageID, tenantID); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) GetWidgetByID(ctx context.Context, id, pageID, tenantID int64) (DashboardRow, error) {
	var row DashboardRow
	q := fmt.Sprintf(`SELECT %s FROM optikk.dashboards
		WHERE id = ? AND page_id = ? AND tenant_id = ? LIMIT 1`, selectWidgetCols)
	if err := dbutil.GetSQL(ctx, r.db, "dashboards.GetWidgetByID", &row, q, id, pageID, tenantID); err != nil {
		return row, err
	}
	return row, nil
}

func (r *Repository) CountWidgets(ctx context.Context, pageID, tenantID int64) (int, error) {
	var n int
	err := dbutil.GetSQL(ctx, r.db, "dashboards.CountWidgets", &n,
		`SELECT COUNT(*) FROM optikk.dashboards WHERE page_id = ? AND tenant_id = ?`, pageID, tenantID)
	return n, err
}

const insertWidget = `
INSERT INTO optikk.dashboards
  (page_id, tenant_id, title, panel_type, layout_variant, spec_json, layout_json,
   position, created_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?)
`

func (r *Repository) CreateWidget(ctx context.Context, row widgetInsertArgs) (int64, error) {
	res, err := dbutil.ExecSQL(ctx, r.db, "dashboards.CreateWidget", insertWidget,
		row.PageID, row.TenantID, row.Title, row.PanelType, row.LayoutVariant,
		row.SpecJSON, row.LayoutJSON, row.Position, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

const updateWidget = `
UPDATE optikk.dashboards
   SET title = ?, panel_type = ?, layout_variant = ?,
       spec_json = ?, layout_json = ?, position = ?, updated_at = ?
 WHERE id = ? AND page_id = ? AND tenant_id = ?
`

func (r *Repository) UpdateWidget(ctx context.Context, id int64, row widgetInsertArgs) error {
	res, err := dbutil.ExecSQL(ctx, r.db, "dashboards.UpdateWidget", updateWidget,
		row.Title, row.PanelType, row.LayoutVariant,
		row.SpecJSON, row.LayoutJSON, row.Position, time.Now().UTC(),
		id, row.PageID, row.TenantID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) DeleteWidget(ctx context.Context, id, pageID, tenantID int64) error {
	res, err := dbutil.ExecSQL(ctx, r.db, "dashboards.DeleteWidget",
		`DELETE FROM optikk.dashboards WHERE id = ? AND page_id = ? AND tenant_id = ?`,
		id, pageID, tenantID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// PageExists confirms a page belongs to the tenant before widget operations.
func (r *Repository) PageExists(ctx context.Context, pageID, tenantID int64) (bool, error) {
	var n int
	err := dbutil.GetSQL(ctx, r.db, "dashboards.PageExists", &n,
		`SELECT COUNT(*) FROM optikk.dashboard_pages WHERE id = ? AND tenant_id = ?`, pageID, tenantID)
	return n > 0, err
}

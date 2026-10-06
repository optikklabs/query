package dashboards

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/shared/filterutil"
	"github.com/optikklabs/query/internal/shared/sqljson"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql")}
}

type pageInsertArgs struct {
	TenantID        int64
	Name            string
	Description     sql.NullString
	Icon            string
	IconColor       string
	Tags            sqljson.StringList
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
		row.Tags, row.IsFavorite, row.CreatedByUserID, time.Now().UTC())
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
	return dbutil.ExecMatched(ctx, r.db, "dashboards.UpdatePage", updatePage,
		row.Name, row.Description, row.Icon, row.IconColor,
		row.Tags, row.IsFavorite, time.Now().UTC(), id, tenantID)
}

// DeletePage removes the page; its widgets go with it (ON DELETE CASCADE).
func (r *Repository) DeletePage(ctx context.Context, id, tenantID int64) error {
	return dbutil.ExecMatched(ctx, r.db, "dashboards.DeletePage",
		`DELETE FROM optikk.dashboard_pages WHERE id = ? AND tenant_id = ?`, id, tenantID)
}

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
	err := dbutil.GetSQL(ctx, r.db, "dashboards.GetPageByID", &row,
		`SELECT `+selectPageCols+` WHERE p.id = ? AND p.tenant_id = ? LIMIT 1`, id, tenantID)
	return row, err
}

func (r *Repository) ListPages(ctx context.Context, tenantID int64, q ListPagesQuery) ([]DashboardPageRow, int, error) {
	where, args := pageFilters(tenantID, q)
	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := "SELECT COUNT(*) FROM optikk.dashboard_pages p WHERE " + whereSQL
	if err := dbutil.GetSQL(ctx, r.db, "dashboards.CountPages", &total, countSQL, args...); err != nil {
		return nil, 0, err
	}

	listArgs := append(slices.Clip(args), q.Limit, q.Offset)
	listSQL := `SELECT ` + selectPageCols + ` WHERE ` + whereSQL + `
		ORDER BY p.is_favorite DESC, p.updated_at DESC, p.created_at DESC, p.id DESC
		LIMIT ? OFFSET ?`

	var rows []DashboardPageRow
	if err := dbutil.SelectSQL(ctx, r.db, "dashboards.ListPages", &rows, listSQL, listArgs...); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func pageFilters(tenantID int64, q ListPagesQuery) ([]string, []any) {
	where := []string{"p.tenant_id = ?"}
	args := []any{tenantID}
	if q.Search != "" {
		where = append(where, "p.name LIKE ?")
		args = append(args, filterutil.LikeSubstringPattern(q.Search))
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

type widgetInsertArgs struct {
	PageID   int64
	TenantID int64
	SpecJSON []byte
	Position int
}

const selectWidgetCols = `
  id, page_id, tenant_id, spec_json, position, created_at, updated_at
`

func (r *Repository) ListWidgets(ctx context.Context, pageID, tenantID int64) ([]DashboardRow, error) {
	var rows []DashboardRow
	err := dbutil.SelectSQL(ctx, r.db, "dashboards.ListWidgets", &rows,
		`SELECT `+selectWidgetCols+` FROM optikk.dashboards
		WHERE page_id = ? AND tenant_id = ?
		ORDER BY position ASC, id ASC`, pageID, tenantID)
	return rows, err
}

func (r *Repository) GetWidgetByID(ctx context.Context, id, pageID, tenantID int64) (DashboardRow, error) {
	var row DashboardRow
	err := dbutil.GetSQL(ctx, r.db, "dashboards.GetWidgetByID", &row,
		`SELECT `+selectWidgetCols+` FROM optikk.dashboards
		WHERE id = ? AND page_id = ? AND tenant_id = ? LIMIT 1`, id, pageID, tenantID)
	return row, err
}

func (r *Repository) CountWidgets(ctx context.Context, pageID, tenantID int64) (int, error) {
	var n int
	err := dbutil.GetSQL(ctx, r.db, "dashboards.CountWidgets", &n,
		`SELECT COUNT(*) FROM optikk.dashboards WHERE page_id = ? AND tenant_id = ?`, pageID, tenantID)
	return n, err
}

const insertWidget = `
INSERT INTO optikk.dashboards
  (page_id, tenant_id, spec_json, position, created_at)
VALUES
  (?, ?, ?, ?, ?)
`

func (r *Repository) CreateWidget(ctx context.Context, row widgetInsertArgs) (int64, error) {
	res, err := dbutil.ExecSQL(ctx, r.db, "dashboards.CreateWidget", insertWidget,
		row.PageID, row.TenantID, row.SpecJSON, row.Position, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

const updateWidget = `
UPDATE optikk.dashboards
   SET spec_json = ?, position = ?, updated_at = ?
 WHERE id = ? AND page_id = ? AND tenant_id = ?
`

func (r *Repository) UpdateWidget(ctx context.Context, id int64, row widgetInsertArgs) error {
	return dbutil.ExecMatched(ctx, r.db, "dashboards.UpdateWidget", updateWidget,
		row.SpecJSON, row.Position, time.Now().UTC(),
		id, row.PageID, row.TenantID)
}

func (r *Repository) DeleteWidget(ctx context.Context, id, pageID, tenantID int64) error {
	return dbutil.ExecMatched(ctx, r.db, "dashboards.DeleteWidget",
		`DELETE FROM optikk.dashboards WHERE id = ? AND page_id = ? AND tenant_id = ?`,
		id, pageID, tenantID)
}

func (r *Repository) PageExists(ctx context.Context, pageID, tenantID int64) (bool, error) {
	var n int
	err := dbutil.GetSQL(ctx, r.db, "dashboards.PageExists", &n,
		`SELECT COUNT(*) FROM optikk.dashboard_pages WHERE id = ? AND tenant_id = ?`, pageID, tenantID)
	return n > 0, err
}

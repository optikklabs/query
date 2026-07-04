// Package dashboards handles CRUD for custom dashboard pages and their widgets.
package dashboards

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"
	"unicode"
)

// DashboardPageRow is a dashboard_pages row, enriched with a derived
// widget_count and the owner name resolved from optikk.users.
type DashboardPageRow struct {
	ID              int64          `db:"id"`
	TenantID        int64          `db:"tenant_id"`
	Name            string         `db:"name"`
	Description     sql.NullString `db:"description"`
	Icon            string         `db:"icon"`
	IconColor       string         `db:"icon_color"`
	TagsJSON        []byte         `db:"tags_json"`
	IsFavorite      bool           `db:"is_favorite"`
	CreatedByUserID sql.NullInt64  `db:"created_by_user_id"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       sql.NullTime   `db:"updated_at"`
	WidgetCount     int            `db:"widget_count"`
	OwnerName       sql.NullString `db:"owner_name"`
}

// DashboardRow is a dashboards (widget) row; its full definition lives in
// spec_json, its grid position in layout_json.
type DashboardRow struct {
	ID            int64          `db:"id"`
	PageID        int64          `db:"page_id"`
	TenantID      int64          `db:"tenant_id"`
	Title         sql.NullString `db:"title"`
	PanelType     string         `db:"panel_type"`
	LayoutVariant sql.NullString `db:"layout_variant"`
	SpecJSON      []byte         `db:"spec_json"`
	LayoutJSON    []byte         `db:"layout_json"`
	Position      int            `db:"position"`
	CreatedAt     time.Time      `db:"created_at"`
	UpdatedAt     sql.NullTime   `db:"updated_at"`
}

// Owner is the page creator, resolved for display in the catalog.
type Owner struct {
	Name     string `json:"name"`
	Initials string `json:"initials"`
}

// DashboardPageResponse is the wire shape for a page in the catalog.
type DashboardPageResponse struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Icon        string     `json:"icon"`
	IconColor   string     `json:"icon_color"`
	Tags        []string   `json:"tags"`
	IsFavorite  bool       `json:"is_favorite"`
	WidgetCount int        `json:"widget_count"`
	Owner       *Owner     `json:"owner,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// DashboardPageListResponse is the catalog list payload.
type DashboardPageListResponse struct {
	Items []DashboardPageResponse `json:"items"`
	Total int                     `json:"total"`
}

// WidgetResponse round-trips the persisted widget definition with no data loss.
type WidgetResponse struct {
	ID            int64           `json:"id"`
	PageID        int64           `json:"page_id"`
	Title         string          `json:"title,omitempty"`
	PanelType     string          `json:"panel_type"`
	LayoutVariant string          `json:"layout_variant,omitempty"`
	Spec          json.RawMessage `json:"spec"`
	Layout        json.RawMessage `json:"layout"`
	Position      int             `json:"position"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     *time.Time      `json:"updated_at,omitempty"`
}

// DashboardPageDetailResponse is a page plus its widgets, for the detail screen.
type DashboardPageDetailResponse struct {
	DashboardPageResponse
	Widgets []WidgetResponse `json:"widgets"`
}

func toPageResponse(row DashboardPageRow) DashboardPageResponse {
	out := DashboardPageResponse{
		ID:          row.ID,
		Name:        row.Name,
		Icon:        row.Icon,
		IconColor:   row.IconColor,
		Tags:        []string{},
		IsFavorite:  row.IsFavorite,
		WidgetCount: row.WidgetCount,
		CreatedAt:   row.CreatedAt,
	}
	if len(row.TagsJSON) > 0 {
		if err := json.Unmarshal(row.TagsJSON, &out.Tags); err != nil {
			out.Tags = []string{}
		}
	}
	if out.Tags == nil {
		out.Tags = []string{}
	}
	if row.Description.Valid {
		out.Description = row.Description.String
	}
	if row.UpdatedAt.Valid {
		t := row.UpdatedAt.Time
		out.UpdatedAt = &t
	}
	if name := strings.TrimSpace(row.OwnerName.String); row.OwnerName.Valid && name != "" {
		out.Owner = &Owner{Name: name, Initials: initials(name)}
	}
	return out
}

func toWidgetResponse(row DashboardRow) WidgetResponse {
	out := WidgetResponse{
		ID:        row.ID,
		PageID:    row.PageID,
		PanelType: row.PanelType,
		Spec:      json.RawMessage(row.SpecJSON),
		Layout:    json.RawMessage(row.LayoutJSON),
		Position:  row.Position,
		CreatedAt: row.CreatedAt,
	}
	if row.Title.Valid {
		out.Title = row.Title.String
	}
	if row.LayoutVariant.Valid {
		out.LayoutVariant = row.LayoutVariant.String
	}
	if row.UpdatedAt.Valid {
		t := row.UpdatedAt.Time
		out.UpdatedAt = &t
	}
	return out
}

// initials derives up to two uppercase initials from a display name.
func initials(name string) string {
	var out []rune
	for _, field := range strings.Fields(name) {
		for _, r := range field {
			out = append(out, unicode.ToUpper(r))
			break
		}
		if len(out) == 2 {
			break
		}
	}
	return string(out)
}

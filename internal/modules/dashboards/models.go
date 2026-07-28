package dashboards

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"
	"unicode"
)

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

type Owner struct {
	Name     string `json:"name"`
	Initials string `json:"initials"`
}

type DashboardPageResponse struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Icon        string     `json:"icon"`
	IconColor   string     `json:"iconColor"`
	Tags        []string   `json:"tags"`
	IsFavorite  bool       `json:"isFavorite"`
	WidgetCount int        `json:"widgetCount"`
	Owner       *Owner     `json:"owner,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
}

type DashboardPageListResponse struct {
	Items []DashboardPageResponse `json:"items"`
	Total int                     `json:"total"`
}

type WidgetResponse struct {
	ID            int64           `json:"id"`
	PageID        int64           `json:"pageId"`
	Title         string          `json:"title,omitempty"`
	PanelType     string          `json:"panelType"`
	LayoutVariant string          `json:"layoutVariant,omitempty"`
	Spec          json.RawMessage `json:"spec"`
	Layout        json.RawMessage `json:"layout"`
	Position      int             `json:"position"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     *time.Time      `json:"updatedAt,omitempty"`
}

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

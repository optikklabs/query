package dashboards

import "encoding/json"

// CreatePageRequest authors a new dashboard page (widgets added separately).
type CreatePageRequest struct {
	Name        string   `json:"name" validate:"required"`
	Description string   `json:"description,omitempty"`
	Icon        string   `json:"icon,omitempty"`
	IconColor   string   `json:"iconColor,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	IsFavorite  bool     `json:"isFavorite"`
}

// UpdatePageRequest reuses the create shape (full replace of editable fields).
type UpdatePageRequest = CreatePageRequest

// CreateWidgetRequest persists a widget's full definition with its query.
type CreateWidgetRequest struct {
	Title         string          `json:"title,omitempty"`
	PanelType     string          `json:"panelType" validate:"required"`
	LayoutVariant string          `json:"layoutVariant,omitempty"`
	Spec          json.RawMessage `json:"spec" validate:"required"`
	Layout        json.RawMessage `json:"layout" validate:"required"`
	Position      int             `json:"position"`
}

// UpdateWidgetRequest reuses the create shape.
type UpdateWidgetRequest = CreateWidgetRequest

// ListPagesQuery carries the catalog filter/pagination params.
type ListPagesQuery struct {
	Search   string
	Favorite bool
	Tag      string
	Limit    int
	Offset   int
}

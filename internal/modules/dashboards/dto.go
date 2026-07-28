package dashboards

import "encoding/json"

type CreatePageRequest struct {
	Name        string   `json:"name" validate:"required"`
	Description string   `json:"description,omitempty"`
	Icon        string   `json:"icon,omitempty"`
	IconColor   string   `json:"iconColor,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	IsFavorite  bool     `json:"isFavorite"`
}

type UpdatePageRequest = CreatePageRequest

type CreateWidgetRequest struct {
	Title         string          `json:"title,omitempty"`
	PanelType     string          `json:"panelType" validate:"required"`
	LayoutVariant string          `json:"layoutVariant,omitempty"`
	Spec          json.RawMessage `json:"spec" validate:"required"`
	Layout        json.RawMessage `json:"layout" validate:"required"`
	Position      int             `json:"position"`
}

type UpdateWidgetRequest = CreateWidgetRequest

type ListPagesQuery struct {
	Search   string
	Favorite bool
	Tag      string
	Limit    int
	Offset   int
}

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
	Spec     json.RawMessage `json:"spec" validate:"required"`
	Position int             `json:"position"`
}

type UpdateWidgetRequest = CreateWidgetRequest

type ListPagesQuery struct {
	Search   string
	Favorite bool
	Tag      string
	Limit    int
	Offset   int
}

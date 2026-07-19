package notifications

import (
	"encoding/json"
	"time"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

// ChannelResponse is the wire shape returned by channels endpoints.
type ChannelResponse struct {
	ID             int64           `json:"id"`
	Type           string          `json:"type"`
	Name           string          `json:"name"`
	Config         json.RawMessage `json:"config"`
	Status         string          `json:"status"`
	UsedByCount    int             `json:"usedByCount"`
	LastUsedAt     *time.Time      `json:"lastUsedAt,omitempty"`
	LastDeliveryAt *time.Time      `json:"lastDeliveryAt,omitempty"`
	LastErrorText  string          `json:"lastErrorText,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
}

type PolicyResponse struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	MatchDSL   string          `json:"matchDsl"`
	Actions    json.RawMessage `json:"actions"`
	Hits30d    int             `json:"hits30d"`
	LastUsedAt *time.Time      `json:"lastUsedAt,omitempty"`
	Enabled    bool            `json:"enabled"`
	Position   int             `json:"position"`
	CreatedAt  time.Time       `json:"createdAt"`
}

type TemplateResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Body        string    `json:"body"`
	UsedCount   int       `json:"usedCount"`
	CreatedAt   time.Time `json:"createdAt"`
}

type IntegrationCatalogEntry struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Desc   string `json:"desc"`
	Status string `json:"status"`
	Count  int    `json:"count"`
	Color  string `json:"color"`
}

func toChannelResponse(row models.ChannelRow, usedBy int) ChannelResponse {
	out := ChannelResponse{
		ID:          row.ID,
		Type:        row.Type,
		Name:        row.Name,
		Config:      row.ConfigJSON,
		Status:      row.Status,
		UsedByCount: usedBy,
		CreatedAt:   row.CreatedAt,
	}
	if row.LastUsedAt.Valid {
		t := row.LastUsedAt.Time
		out.LastUsedAt = &t
	}
	if row.LastDeliveryAt.Valid {
		t := row.LastDeliveryAt.Time
		out.LastDeliveryAt = &t
	}
	if row.LastErrorText.Valid {
		out.LastErrorText = row.LastErrorText.String
	}
	return out
}

func toPolicyResponse(row models.PolicyRow) PolicyResponse {
	out := PolicyResponse{
		ID:        row.ID,
		Name:      row.Name,
		MatchDSL:  row.MatchDSL,
		Actions:   row.ActionsJSON,
		Hits30d:   row.Hits30d,
		Enabled:   row.Enabled,
		Position:  row.Position,
		CreatedAt: row.CreatedAt,
	}
	if row.LastUsedAt.Valid {
		t := row.LastUsedAt.Time
		out.LastUsedAt = &t
	}
	return out
}

func toTemplateResponse(row models.TemplateRow) TemplateResponse {
	out := TemplateResponse{
		ID:        row.ID,
		Name:      row.Name,
		Body:      row.Body,
		UsedCount: row.UsedCount,
		CreatedAt: row.CreatedAt,
	}
	if row.Description.Valid {
		out.Description = row.Description.String
	}
	return out
}

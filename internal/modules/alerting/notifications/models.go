package notifications

import (
	"encoding/json"
	"strings"
	"time"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/shared/nullable"
)

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
	return ChannelResponse{
		ID:             row.ID,
		Type:           row.Type,
		Name:           row.Name,
		Config:         publicChannelConfig(row),
		Status:         row.Status,
		UsedByCount:    usedBy,
		LastUsedAt:     nullable.Ptr(row.LastUsedAt.Time, row.LastUsedAt.Valid),
		LastDeliveryAt: nullable.Ptr(row.LastDeliveryAt.Time, row.LastDeliveryAt.Valid),
		LastErrorText:  row.LastErrorText.String,
		CreatedAt:      row.CreatedAt,
	}
}

func publicChannelConfig(row models.ChannelRow) json.RawMessage {
	configured := false
	if row.Type == "slack" {
		var cfg models.SlackWebhookConfig
		configured = json.Unmarshal(row.ConfigJSON, &cfg) == nil && strings.TrimSpace(cfg.WebhookURL) != ""
	}
	if configured {
		return json.RawMessage(`{"webhookConfigured":true}`)
	}
	return json.RawMessage(`{"webhookConfigured":false}`)
}

func toPolicyResponse(row models.PolicyRow) PolicyResponse {
	return PolicyResponse{
		ID:         row.ID,
		Name:       row.Name,
		MatchDSL:   row.MatchDSL,
		Actions:    row.ActionsJSON,
		Hits30d:    row.Hits30d,
		LastUsedAt: nullable.Ptr(row.LastUsedAt.Time, row.LastUsedAt.Valid),
		Enabled:    row.Enabled,
		Position:   row.Position,
		CreatedAt:  row.CreatedAt,
	}
}

func toTemplateResponse(row models.TemplateRow) TemplateResponse {
	return TemplateResponse{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description.String,
		Body:        row.Body,
		UsedCount:   row.UsedCount,
		CreatedAt:   row.CreatedAt,
	}
}

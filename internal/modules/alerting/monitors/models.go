package monitors

import (
	"time"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/shared/nullable"
)

type MonitorResponse struct {
	ID               int64                `json:"id"`
	Name             string               `json:"name"`
	Type             string               `json:"type"`
	Priority         string               `json:"priority"`
	Status           string               `json:"status"`
	CurrentValue     *float64             `json:"currentValue,omitempty"`
	Scope            models.Scope         `json:"scope"`
	Query            models.MonitorQuery  `json:"query"`
	Conditions       models.Conditions    `json:"conditions"`
	Notify           models.NotifyTargets `json:"notify"`
	MessageBody      string               `json:"messageBody,omitempty"`
	RunbookURL       string               `json:"runbookUrl,omitempty"`
	Tags             []string             `json:"tags"`
	EvalEverySec     int                  `json:"evalEverySec"`
	RenotifyEverySec *int                 `json:"renotifyEverySec,omitempty"`
	MutedUntil       *time.Time           `json:"mutedUntil,omitempty"`
	Active           bool                 `json:"active"`
	LastEvaluatedAt  *time.Time           `json:"lastEvaluatedAt,omitempty"`
	TriggeredAt      *time.Time           `json:"triggeredAt,omitempty"`
	CreatedAt        time.Time            `json:"createdAt"`
	UpdatedAt        *time.Time           `json:"updatedAt,omitempty"`
}

type MonitorListResponse struct {
	Items  []MonitorResponse `json:"items"`
	Counts StatusCounts      `json:"counts"`
}

type StatusCounts struct {
	Alert  int `json:"alert" db:"alert"`
	Warn   int `json:"warn" db:"warn"`
	OK     int `json:"ok" db:"ok"`
	NoData int `json:"noData" db:"no_data"`
	Muted  int `json:"muted" db:"muted"`
	Total  int `json:"total" db:"total"`
}

func toResponse(row models.MonitorRow, state models.MonitorStateRow) MonitorResponse {
	out := MonitorResponse{
		ID:               row.ID,
		Name:             row.Name,
		Type:             row.Type,
		Priority:         row.Priority,
		Status:           "no_data",
		Scope:            row.Scope,
		Query:            row.Query,
		Conditions:       row.Conditions,
		Notify:           row.Notify,
		MessageBody:      row.MessageBody.String,
		RunbookURL:       row.RunbookURL.String,
		Tags:             nullable.OrEmpty(row.Tags),
		EvalEverySec:     row.EvalEverySec,
		RenotifyEverySec: nullable.Ptr(int(row.RenotifyEverySec.Int64), row.RenotifyEverySec.Valid),
		MutedUntil:       nullable.Ptr(row.MutedUntil.Time, row.MutedUntil.Valid),
		Active:           row.Active,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        nullable.Ptr(row.UpdatedAt.Time, row.UpdatedAt.Valid),
	}
	if state.MonitorID != 0 {
		out.Status = state.Status
		out.CurrentValue = nullable.Ptr(state.CurrentValue.Float64, state.CurrentValue.Valid)
		out.LastEvaluatedAt = nullable.Ptr(state.LastEvaluatedAt.Time, state.LastEvaluatedAt.Valid)
		out.TriggeredAt = nullable.Ptr(state.TriggeredAt.Time, state.TriggeredAt.Valid)
	}
	return out
}

package monitors

import (
	"time"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
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
		ID:           row.ID,
		Name:         row.Name,
		Type:         row.Type,
		Priority:     row.Priority,
		EvalEverySec: row.EvalEverySec,
		Active:       row.Active,
		CreatedAt:    row.CreatedAt,
		Status:       "no_data",
		Tags:         row.Tags,
		Scope:        row.Scope,
		Query:        row.Query,
		Conditions:   row.Conditions,
		Notify:       row.Notify,
	}
	if out.Tags == nil {
		out.Tags = []string{}
	}
	if row.MessageBody.Valid {
		out.MessageBody = row.MessageBody.String
	}
	if row.RunbookURL.Valid {
		out.RunbookURL = row.RunbookURL.String
	}
	if row.RenotifyEverySec.Valid {
		v := int(row.RenotifyEverySec.Int64)
		out.RenotifyEverySec = &v
	}
	if row.MutedUntil.Valid {
		t := row.MutedUntil.Time
		out.MutedUntil = &t
	}
	if row.UpdatedAt.Valid {
		t := row.UpdatedAt.Time
		out.UpdatedAt = &t
	}
	if state.MonitorID != 0 {
		out.Status = state.Status
		if state.CurrentValue.Valid {
			v := state.CurrentValue.Float64
			out.CurrentValue = &v
		}
		if state.LastEvaluatedAt.Valid {
			t := state.LastEvaluatedAt.Time
			out.LastEvaluatedAt = &t
		}
		if state.TriggeredAt.Valid {
			t := state.TriggeredAt.Time
			out.TriggeredAt = &t
		}
	}
	return out
}

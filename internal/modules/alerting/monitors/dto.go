package monitors

import (
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

type CreateMonitorRequest struct {
	Name             string               `json:"name" validate:"required"`
	Type             string               `json:"type" validate:"required"`
	Priority         string               `json:"priority"`
	Scope            models.Scope         `json:"scope"`
	Query            models.MonitorQuery  `json:"query"`
	Conditions       models.Conditions    `json:"conditions"`
	Notify           models.NotifyTargets `json:"notify"`
	MessageBody      string               `json:"messageBody,omitempty"`
	RunbookURL       string               `json:"runbookUrl,omitempty"`
	Tags             []string             `json:"tags,omitempty"`
	EvalEverySec     int                  `json:"evalEverySec"`
	RenotifyEverySec *int                 `json:"renotifyEverySec,omitempty"`
}

type UpdateMonitorRequest = CreateMonitorRequest

type MuteRequest struct {
	DurationSec int `json:"durationSec" validate:"required"`
}

type ListQuery struct {
	Status string

	Type     string
	Priority string

	Muted  *bool
	Search string
	Limit  int
	Offset int
}

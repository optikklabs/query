// Package monitors handles CRUD, state actions, and history for monitors.
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
	MessageBody      string               `json:"message_body,omitempty"`
	RunbookURL       string               `json:"runbook_url,omitempty"`
	Tags             []string             `json:"tags,omitempty"`
	EvalEverySec     int                  `json:"eval_every_sec"`
	RenotifyEverySec *int                 `json:"renotify_every_sec,omitempty"`
}

type UpdateMonitorRequest = CreateMonitorRequest

type MuteRequest struct {
	DurationSec int `json:"duration_sec" validate:"required"`
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

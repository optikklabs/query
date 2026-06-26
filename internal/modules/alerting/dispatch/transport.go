// Package dispatch sends notification payloads to notification channels.
// Transports route payloads depending on their channel type.
package dispatch

import (
	"context"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

type Payload struct {
	MonitorID   int64
	MonitorName string
	MonitorURL  string

	MonitorType string

	Priority string

	Transition string

	Status       string
	Value        float64
	Threshold    float64
	ScopeSummary string

	Message    string
	IsAlert    bool
	IsWarning  bool
	IsRecovery bool
}

type Transport interface {
	Send(ctx context.Context, ch models.ChannelRow, p Payload) error
}

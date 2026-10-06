package dispatch

import (
	"context"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

type UnsupportedChannelTypeError struct {
	Type string
}

func (e UnsupportedChannelTypeError) Error() string {
	return "unsupported notification channel type: " + e.Type
}

type Dispatcher struct {
	slack *SlackWebhook
}

func NewDefaultDispatcher() *Dispatcher {
	return &Dispatcher{
		slack: NewSlackWebhook(),
	}
}

func (d *Dispatcher) Dispatch(ctx context.Context, ch models.ChannelRow, p Payload) error {
	if ch.Type == "slack" {
		return d.slack.Send(ctx, ch, p)
	}
	return UnsupportedChannelTypeError{Type: ch.Type}
}

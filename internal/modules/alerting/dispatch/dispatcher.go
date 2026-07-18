package dispatch

import (
	"context"
	"fmt"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

// UnsupportedChannelTypeError reports a channel without a real transport.
type UnsupportedChannelTypeError struct {
	Type string
}

func (e UnsupportedChannelTypeError) Error() string {
	return fmt.Sprintf("unsupported notification channel type: %s", e.Type)
}

// Dispatcher routes alert payloads to implemented transports.
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

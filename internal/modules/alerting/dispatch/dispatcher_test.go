package dispatch

import (
	"context"
	"errors"
	"testing"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

func TestDispatcherRejectsUnsupportedChannelType(t *testing.T) {
	d := NewDefaultDispatcher()
	err := d.Dispatch(context.Background(), models.ChannelRow{Type: "pagerduty"}, Payload{})

	var unsupported UnsupportedChannelTypeError
	if !errors.As(err, &unsupported) {
		t.Fatalf("Dispatch error = %v, want UnsupportedChannelTypeError", err)
	}
	if unsupported.Type != "pagerduty" {
		t.Fatalf("unsupported type = %q, want pagerduty", unsupported.Type)
	}
}

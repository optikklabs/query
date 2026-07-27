package notifications

import (
	"encoding/json"
	"strings"
	"testing"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

func TestBuildChannelRowAcceptsOnlyImplementedTypes(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateChannelRequest
		wantErr bool
	}{
		{
			name: "slack",
			req: CreateChannelRequest{
				Type: "slack", Name: "on-call",
				Config: []byte(`{"webhookUrl":"https://hooks.slack.test/1"}`),
			},
		},
		{
			name:    "unsupported provider",
			req:     CreateChannelRequest{Type: "pagerduty", Name: "on-call"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildChannelRow(7, tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("buildChannelRow error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestChannelResponseDoesNotExposeWebhook(t *testing.T) {
	const secret = "https://hooks.slack.test/secret"
	response := toChannelResponse(models.ChannelRow{
		Type:       "slack",
		ConfigJSON: json.RawMessage(`{"webhookUrl":"` + secret + `"}`),
	}, 0)

	if strings.Contains(string(response.Config), secret) || strings.Contains(string(response.Config), "webhookUrl") {
		t.Fatalf("response config exposed webhook: %s", response.Config)
	}
	if got := string(response.Config); got != `{"webhookConfigured":true}` {
		t.Fatalf("response config = %s", got)
	}
}

func TestUpdateChannelKeepsExistingWebhookWhenOmitted(t *testing.T) {
	const secret = "https://hooks.slack.test/secret"
	req, err := preserveChannelCredentials(models.ChannelRow{
		Type:       "slack",
		ConfigJSON: json.RawMessage(`{"webhookUrl":"` + secret + `"}`),
	}, UpdateChannelRequest{
		Type:   "slack",
		Name:   "renamed",
		Config: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(req.Config), secret) {
		t.Fatalf("stored webhook was not preserved: %s", req.Config)
	}
}

func TestIntegrationCatalogMatchesImplementedChannelTypes(t *testing.T) {
	if len(integrationCatalog) != len(models.ChannelTypes) {
		t.Fatalf("catalog size = %d, channel types = %d", len(integrationCatalog), len(models.ChannelTypes))
	}
	for i, channelType := range models.ChannelTypes {
		if integrationCatalog[i].ID != channelType {
			t.Fatalf("catalog[%d] = %q, want %q", i, integrationCatalog[i].ID, channelType)
		}
	}
}

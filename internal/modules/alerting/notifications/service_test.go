package notifications

import (
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

package notifications

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"strings"
	"time"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/alerting/dispatch"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

type Service struct {
	repo       *Repository
	dispatcher *dispatch.Dispatcher
}

func NewService(repo *Repository, dispatcher *dispatch.Dispatcher) *Service {
	return &Service{repo: repo, dispatcher: dispatcher}
}

var (
	ErrNotFound     = errorcode.NotFoundError{Msg: "resource not found"}
	ErrChannelInUse = errorcode.ConflictError{Msg: "channel is in use by one or more monitors"}
)

func (s *Service) CreateChannel(ctx context.Context, tenantID int64, req CreateChannelRequest) (ChannelResponse, error) {
	row, err := buildChannelRow(tenantID, req)
	if err != nil {
		return ChannelResponse{}, err
	}
	id, err := s.repo.CreateChannel(ctx, row)
	if err != nil {
		return ChannelResponse{}, err
	}
	return s.GetChannel(ctx, tenantID, id)
}

func (s *Service) UpdateChannel(ctx context.Context, tenantID, id int64, req UpdateChannelRequest) (ChannelResponse, error) {
	existing, err := s.repo.GetChannel(ctx, id, tenantID)
	if err != nil {
		return ChannelResponse{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	req, err = preserveChannelCredentials(existing, req)
	if err != nil {
		return ChannelResponse{}, err
	}
	row, err := buildChannelRow(tenantID, req)
	if err != nil {
		return ChannelResponse{}, err
	}
	if err := s.repo.UpdateChannel(ctx, id, tenantID, row); err != nil {
		return ChannelResponse{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	return s.GetChannel(ctx, tenantID, id)
}

func preserveChannelCredentials(existing models.ChannelRow, req UpdateChannelRequest) (UpdateChannelRequest, error) {
	var next models.SlackWebhookConfig
	if len(req.Config) > 0 {
		if err := json.Unmarshal(req.Config, &next); err != nil {
			return req, errorcode.ValidationError{Msg: "config must be a JSON object"}
		}
	}
	if next.WebhookURL == "" {
		// Responses never echo the webhook URL, so an edit that omits it
		// keeps the stored one.
		req.Config = existing.ConfigJSON
	}
	return req, nil
}

func (s *Service) DeleteChannel(ctx context.Context, tenantID, id int64) error {
	inUse, err := s.repo.ChannelInUse(ctx, id, tenantID)
	if err != nil {
		return err
	}
	if inUse {
		return ErrChannelInUse
	}
	return dbutil.NoRowsAs(s.repo.DeleteChannel(ctx, id, tenantID), ErrNotFound)
}

func (s *Service) GetChannel(ctx context.Context, tenantID, id int64) (ChannelResponse, error) {
	row, err := s.repo.GetChannel(ctx, id, tenantID)
	if err != nil {
		return ChannelResponse{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	usage, err := s.repo.CountChannelUsage(ctx, tenantID)
	if err != nil {
		return ChannelResponse{}, err
	}
	return toChannelResponse(row, usage[row.ID]), nil
}

func (s *Service) ListChannels(ctx context.Context, tenantID int64) ([]ChannelResponse, error) {
	rows, err := s.repo.ListChannels(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	usage, err := s.repo.CountChannelUsage(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]ChannelResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toChannelResponse(row, usage[row.ID]))
	}
	return out, nil
}

// TestChannel sends a sample alert through the channel and records the
// delivery outcome on it, exactly as a real notification would.
func (s *Service) TestChannel(ctx context.Context, tenantID, id int64) (TestChannelResponse, error) {
	row, err := s.repo.GetChannel(ctx, id, tenantID)
	if err != nil {
		return TestChannelResponse{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	payload := dispatch.Payload{
		MonitorName:  "[Test] Optikk Monitors delivery",
		Priority:     "P3",
		Transition:   "ok->alert",
		Status:       "alert",
		Value:        0.42,
		Threshold:    0.05,
		ScopeSummary: "test channel · no real monitor",
		Message:      "If you can see this message, this channel is wired correctly.",
		IsAlert:      true,
	}
	out := TestChannelResponse{OK: true}
	deliveryErr := s.dispatcher.Dispatch(ctx, row, payload)
	if deliveryErr != nil {
		out = TestChannelResponse{ErrorText: deliveryErr.Error()}
	}
	if err := s.repo.MarkChannelDelivered(ctx, id, time.Now().UTC(), deliveryErr); err != nil {
		slog.WarnContext(ctx, "notifications: record test delivery failed", slog.Int64("channel_id", id), slog.Any("error", err))
	}
	return out, nil
}

func buildChannelRow(tenantID int64, req CreateChannelRequest) (models.ChannelRow, error) {
	t := strings.TrimSpace(req.Type)
	if !models.IsValidChannelType(t) {
		return models.ChannelRow{}, errorcode.ValidationError{Msg: "type must be one of " + strings.Join(models.ChannelTypes, ", ")}
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return models.ChannelRow{}, errorcode.ValidationError{Msg: "name is required"}
	}
	var sc models.SlackWebhookConfig
	if err := json.Unmarshal(req.Config, &sc); err != nil {
		return models.ChannelRow{}, errorcode.ValidationError{Msg: "config must be a JSON object"}
	}
	if err := validateSlackWebhook(sc.WebhookURL); err != nil {
		return models.ChannelRow{}, err
	}
	cfg, err := json.Marshal(sc)
	if err != nil {
		return models.ChannelRow{}, err
	}
	return models.ChannelRow{
		TenantID:   tenantID,
		Type:       t,
		Name:       name,
		ConfigJSON: cfg,
	}, nil
}

// slackWebhookPrefix is the only destination a Slack channel may post to, so a
// channel cannot make the server call internal addresses.
const slackWebhookPrefix = "https://hooks.slack.com/services/"

func validateSlackWebhook(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || !strings.HasPrefix(raw, slackWebhookPrefix) || u.User != nil || len(u.Path) <= len("/services/") {
		return errorcode.ValidationError{Msg: "config.webhookUrl must be a Slack incoming webhook (" + slackWebhookPrefix + "...)"}
	}
	return nil
}

var integrationCatalog = []struct {
	ID    string
	Name  string
	Desc  string
	Color string
}{
	{"slack", "Slack", "Send rich messages to channels", "#611f69"},
}

func (s *Service) ListIntegrations(ctx context.Context, tenantID int64) ([]IntegrationCatalogEntry, error) {
	rows, err := s.repo.ListChannels(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, row := range rows {
		counts[row.Type]++
	}
	out := make([]IntegrationCatalogEntry, 0, len(integrationCatalog))
	for _, it := range integrationCatalog {
		status := "not_connected"
		if counts[it.ID] > 0 {
			status = "connected"
		}
		out = append(out, IntegrationCatalogEntry{
			ID: it.ID, Name: it.Name, Desc: it.Desc, Color: it.Color,
			Status: status, Count: counts[it.ID],
		})
	}
	return out, nil
}

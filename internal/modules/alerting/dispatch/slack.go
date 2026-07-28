package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

type SlackWebhook struct {
	client *http.Client
}

func NewSlackWebhook() *SlackWebhook {
	return &SlackWebhook{client: &http.Client{Timeout: 5 * time.Second}}
}

type slackAttachment struct {
	Color    string       `json:"color"`
	Pretext  string       `json:"pretext,omitempty"`
	Title    string       `json:"title"`
	TitleURL string       `json:"titleLink,omitempty"`
	Text     string       `json:"text,omitempty"`
	Fields   []slackField `json:"fields,omitempty"`
	Footer   string       `json:"footer,omitempty"`
	Ts       int64        `json:"ts,omitempty"`
}

type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

type slackBody struct {
	Attachments []slackAttachment `json:"attachments"`
}

func (s *SlackWebhook) Send(ctx context.Context, ch models.ChannelRow, p Payload) error {
	var cfg models.SlackWebhookConfig
	if err := json.Unmarshal(ch.ConfigJSON, &cfg); err != nil {
		return fmt.Errorf("invalid slack config: %w", err)
	}
	if cfg.WebhookURL == "" {
		return fmt.Errorf("slack channel %d missing webhookUrl", ch.ID)
	}

	body := slackBody{Attachments: []slackAttachment{buildAttachment(p)}}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	return s.post(ctx, cfg.WebhookURL, raw)
}

func (s *SlackWebhook) post(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack webhook request is invalid")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {

		return fmt.Errorf("slack webhook request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("slack webhook returned %d", resp.StatusCode)
	}
	return nil
}

func buildAttachment(p Payload) slackAttachment {
	color := "#22c55e"
	switch p.Status {
	case "alert":
		color = "#ef4444"
	case "warn":
		color = "#f59e0b"
	case "no_data":
		color = "#6b7280"
	}
	pretext := p.Transition
	if p.IsRecovery {
		pretext = "Recovered: " + pretext
	} else if p.IsAlert {
		pretext = "Alerting: " + pretext
	} else if p.IsWarning {
		pretext = "Warning: " + pretext
	}
	att := slackAttachment{
		Color:    color,
		Pretext:  pretext,
		Title:    p.MonitorName,
		TitleURL: p.MonitorURL,
		Text:     p.Message,
		Footer:   "Optikk Monitors · " + p.Priority,
		Ts:       time.Now().Unix(),
	}
	att.Fields = []slackField{
		{Title: "Value", Value: fmt.Sprintf("%g", p.Value), Short: true},
		{Title: "Threshold", Value: fmt.Sprintf("%g", p.Threshold), Short: true},
	}
	if p.ScopeSummary != "" {
		att.Fields = append(att.Fields, slackField{Title: "Scope", Value: p.ScopeSummary, Short: false})
	}
	return att
}

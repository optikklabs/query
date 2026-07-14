package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type PasswordResetSender interface {
	SendPasswordReset(ctx context.Context, to, token string) error
}

type ResendPasswordResetSender struct {
	apiKey     string
	from       string
	baseURL    string
	httpClient *http.Client
}

func NewResendPasswordResetSender(apiKey, from, baseURL string) *ResendPasswordResetSender {
	return &ResendPasswordResetSender{
		apiKey:     apiKey,
		from:       from,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

type noopPasswordResetSender struct{}

func (noopPasswordResetSender) SendPasswordReset(context.Context, string, string) error {
	return nil
}

func (s *ResendPasswordResetSender) SendPasswordReset(ctx context.Context, to, token string) error {
	resetURL := s.baseURL + "?token=" + url.QueryEscape(token)
	body, _ := json.Marshal(map[string]any{
		"from":    s.from,
		"to":      []string{to},
		"subject": "Reset your Optikk password",
		"html":    fmt.Sprintf(`<p>You requested to reset your password. Click <a href="%s">this link</a> to set a new password. This link expires in 30 minutes.</p>`, resetURL),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("email provider returned %s: %s", resp.Status, string(bodyBytes))
	}
	return nil
}

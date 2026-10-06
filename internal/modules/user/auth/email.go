package auth

import (
	"context"
	"html"
	"net/url"

	emailinfra "github.com/optikklabs/query/internal/infra/email"
)

type PasswordResetSender interface {
	SendPasswordReset(ctx context.Context, to, token string) error
}

type ResendPasswordResetSender struct {
	baseURL string
	mailer  *emailinfra.ResendSender
}

func NewResendPasswordResetSender(apiKey, from, baseURL string) *ResendPasswordResetSender {
	return &ResendPasswordResetSender{
		baseURL: baseURL,
		mailer:  emailinfra.NewResendSender(apiKey, from),
	}
}

type noopPasswordResetSender struct{}

func (noopPasswordResetSender) SendPasswordReset(context.Context, string, string) error {
	return nil
}

func (s *ResendPasswordResetSender) SendPasswordReset(ctx context.Context, to, resetToken string) error {
	resetURL := s.baseURL + "?token=" + url.QueryEscape(resetToken)
	body := `<p>You requested to reset your password. Click <a href="` + html.EscapeString(resetURL) + `">this link</a> to set a new password. This link expires in 30 minutes.</p>`
	return s.mailer.Send(ctx, to, "Reset your Optikk password", body)
}

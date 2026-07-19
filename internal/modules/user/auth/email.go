package auth

import (
	"context"
	"fmt"
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

func (s *ResendPasswordResetSender) SendPasswordReset(ctx context.Context, to, token string) error {
	resetURL := s.baseURL + "?token=" + url.QueryEscape(token)
	html := fmt.Sprintf(`<p>You requested to reset your password. Click <a href="%s">this link</a> to set a new password. This link expires in 30 minutes.</p>`, resetURL)
	return s.mailer.Send(ctx, to, "Reset your Optikk password", html)
}

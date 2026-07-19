package email

import (
	"context"
	"net/http"
	"time"

	resend "github.com/resend/resend-go/v3"
)

type resendEmails interface {
	SendWithContext(context.Context, *resend.SendEmailRequest) (*resend.SendEmailResponse, error)
}

// ResendSender sends transactional email through the Resend SDK.
type ResendSender struct {
	emails resendEmails
	from   string
}

func NewResendSender(apiKey, from string) *ResendSender {
	client := resend.NewCustomClient(&http.Client{Timeout: 5 * time.Second}, apiKey)
	return &ResendSender{emails: client.Emails, from: from}
}

func (s *ResendSender) Send(ctx context.Context, to, subject, html string) error {
	_, err := s.emails.SendWithContext(ctx, &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{to},
		Subject: subject,
		Html:    html,
	})
	return err
}

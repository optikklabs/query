package email

import (
	"context"
	"errors"
	"testing"

	resend "github.com/resend/resend-go/v3"
)

type fakeResendEmails struct {
	request *resend.SendEmailRequest
	err     error
}

func (f *fakeResendEmails) SendWithContext(_ context.Context, request *resend.SendEmailRequest) (*resend.SendEmailResponse, error) {
	f.request = request
	return &resend.SendEmailResponse{Id: "email-id"}, f.err
}

func TestResendSenderSend(t *testing.T) {
	client := &fakeResendEmails{}
	sender := &ResendSender{emails: client, from: "Optikk <hello@example.com>"}

	if err := sender.Send(context.Background(), "user@example.com", "Subject", "<p>Hello</p>"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if client.request.From != "Optikk <hello@example.com>" || client.request.To[0] != "user@example.com" {
		t.Fatalf("Send() request = %#v", client.request)
	}
	if client.request.Subject != "Subject" || client.request.Html != "<p>Hello</p>" {
		t.Fatalf("Send() content = %#v", client.request)
	}
}

func TestResendSenderReturnsSDKError(t *testing.T) {
	want := errors.New("resend failed")
	sender := &ResendSender{emails: &fakeResendEmails{err: want}, from: "hello@example.com"}

	if err := sender.Send(context.Background(), "user@example.com", "Subject", "body"); !errors.Is(err, want) {
		t.Fatalf("Send() error = %v, want %v", err, want)
	}
}

package dispatch

import (
	"context"
	"strings"
	"testing"
)

func TestSlackRequestErrorsDoNotExposeWebhook(t *testing.T) {
	const secret = "credential-token"
	err := NewSlackWebhook().post(context.Background(), "://"+secret, nil)
	if err == nil {
		t.Fatal("expected invalid request error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error exposed webhook credential: %v", err)
	}
}

package signup

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/optikklabs/query/internal/config"
)

func TestValidateSignup(t *testing.T) {
	cases := []struct {
		name                           string
		email, uname, tenant, password string
		accepted                       bool
		wantErr                        bool
	}{
		{"valid", "a@b.com", "Ada", "Acme", "longenough", true, false},
		{"missing email", "", "Ada", "Acme", "longenough", true, true},
		{"bad email", "nope", "Ada", "Acme", "longenough", true, true},
		{"missing name", "a@b.com", "", "Acme", "longenough", true, true},
		{"missing tenant", "a@b.com", "Ada", "", "longenough", true, true},
		{"short password", "a@b.com", "Ada", "Acme", "short", true, true},
		{"terms not accepted", "a@b.com", "Ada", "Acme", "longenough", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateSignup(normalizedSignup{
				email: c.email, name: c.uname, tenantName: c.tenant,
				password: c.password, acceptedTerms: c.accepted,
			})
			if (err != nil) != c.wantErr {
				t.Fatalf("validateSignup(%q) err=%v, wantErr=%v", c.name, err, c.wantErr)
			}
		})
	}
}

func TestIsDuplicateEmail(t *testing.T) {
	if !IsDuplicateEmail(&mysql.MySQLError{Number: mysqlDuplicateEntry}) {
		t.Fatal("expected 1062 to be detected as duplicate")
	}
	if IsDuplicateEmail(&mysql.MySQLError{Number: 1234}) {
		t.Fatal("non-1062 must not be a duplicate")
	}
	if IsDuplicateEmail(errors.New("plain error")) {
		t.Fatal("plain error must not be a duplicate")
	}
}

func TestNewServiceVerificationMode(t *testing.T) {
	enabled := NewService(nil, nil, config.EmailConfig{ResendVerificationEnabled: true})
	if !enabled.verificationRequired {
		t.Fatal("expected resend-enabled service to require verification")
	}
	if _, ok := enabled.sender.(*ResendVerificationSender); !ok {
		t.Fatalf("expected resend sender, got %T", enabled.sender)
	}

	disabled := NewService(nil, nil, config.EmailConfig{ResendVerificationEnabled: false})
	if disabled.verificationRequired {
		t.Fatal("expected resend-disabled service to skip verification")
	}
	if _, ok := disabled.sender.(noopVerificationSender); !ok {
		t.Fatalf("expected noop sender, got %T", disabled.sender)
	}
}

func TestPrepareSignupSecretsSkipsVerificationTokenWhenDisabled(t *testing.T) {
	s := NewService(nil, nil, config.EmailConfig{ResendVerificationEnabled: false})
	secrets, err := s.prepareSignupSecrets("longenough")
	if err != nil {
		t.Fatalf("prepareSignupSecrets() err = %v", err)
	}
	if secrets.apiKey == "" {
		t.Fatal("expected api key")
	}
	if secrets.verificationToken != "" || secrets.verificationHash != "" || !secrets.verificationExpiry.IsZero() {
		t.Fatal("expected verification fields to stay empty when verification is disabled")
	}
}

func TestPrepareSignupSecretsIncludesVerificationTokenWhenEnabled(t *testing.T) {
	s := NewService(nil, nil, config.EmailConfig{ResendVerificationEnabled: true})
	secrets, err := s.prepareSignupSecrets("longenough")
	if err != nil {
		t.Fatalf("prepareSignupSecrets() err = %v", err)
	}
	if secrets.apiKey == "" {
		t.Fatal("expected api key")
	}
	if secrets.verificationToken == "" || secrets.verificationHash == "" || secrets.verificationExpiry.IsZero() {
		t.Fatal("expected verification fields when verification is enabled")
	}
}

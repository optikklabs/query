package signup

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestValidateSignup(t *testing.T) {
	cases := []struct {
		name                           string
		email, uname, tenant, password string
		wantErr                        bool
	}{
		{"valid", "a@b.com", "Ada", "Acme", "longenough", false},
		{"missing email", "", "Ada", "Acme", "longenough", true},
		{"bad email", "nope", "Ada", "Acme", "longenough", true},
		{"missing name", "a@b.com", "", "Acme", "longenough", true},
		{"missing tenant", "a@b.com", "Ada", "", "longenough", true},
		{"short password", "a@b.com", "Ada", "Acme", "short", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateSignup(c.email, c.uname, c.tenant, c.password)
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

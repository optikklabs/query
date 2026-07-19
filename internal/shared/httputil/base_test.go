package httputil

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONStrict(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	tests := []struct {
		name string
		body string
		ok   bool
	}{
		{name: "valid", body: `{"name":"api"}`, ok: true},
		{name: "unknown field", body: `{"name":"api","extra":true}`},
		{name: "multiple values", body: `{"name":"api"} {"name":"worker"}`},
		{name: "malformed", body: `{"name":`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", strings.NewReader(test.body))
			var got payload
			err := DecodeJSON(req, &got)
			if (err == nil) != test.ok {
				t.Fatalf("error = %v, ok=%v", err, test.ok)
			}
		})
	}
}

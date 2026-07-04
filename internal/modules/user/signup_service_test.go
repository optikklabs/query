package user

import (
	"testing"
	"time"
)

func TestValidateSignup(t *testing.T) {
	valid := SignupRequest{Email: "founder@startup.dev", Password: "hunter2hunter2", Name: "Founder", OrgName: "Startup"}

	tests := []struct {
		name    string
		mutate  func(r *SignupRequest)
		wantErr bool
	}{
		{"valid", func(r *SignupRequest) {}, false},
		{"bad email", func(r *SignupRequest) { r.Email = "not-an-email" }, true},
		{"empty email", func(r *SignupRequest) { r.Email = "" }, true},
		{"short password", func(r *SignupRequest) { r.Password = "short" }, true},
		{"missing name", func(r *SignupRequest) { r.Name = "" }, true},
		{"missing org", func(r *SignupRequest) { r.OrgName = "" }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := valid
			tt.mutate(&req)
			if err := validateSignup(req); (err != nil) != tt.wantErr {
				t.Errorf("validateSignup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignupLimiter(t *testing.T) {
	l := newSignupLimiter(2, time.Hour)

	for i := 0; i < 2; i++ {
		if !l.Allow("1.1.1.1") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if l.Allow("1.1.1.1") {
		t.Error("third request should be limited")
	}
	if !l.Allow("2.2.2.2") {
		t.Error("other IPs must not share the window")
	}

	// Expired windows reset the counter.
	l.byIP["1.1.1.1"].startAt = time.Now().Add(-2 * time.Hour)
	if !l.Allow("1.1.1.1") {
		t.Error("expired window should allow again")
	}
}

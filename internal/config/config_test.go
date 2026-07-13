package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func validConfig() Config {
	var c Config
	c.Auth.JWTSecret = strings.Repeat("x", minJWTSecretLen)
	c.MySQL.Password = "pw"
	c.ClickHouse.Password = "pw"
	c.Email.ResendVerificationEnabled = true
	return c
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"valid", func(*Config) {}, false},
		{"empty secret", func(c *Config) { c.Auth.JWTSecret = "" }, true},
		{"short secret", func(c *Config) { c.Auth.JWTSecret = strings.Repeat("x", minJWTSecretLen-1) }, true},
		{"empty mysql password", func(c *Config) { c.MySQL.Password = "" }, true},
		{"empty clickhouse password", func(c *Config) { c.ClickHouse.Password = "" }, true},
		{"production resend enabled requires email config", func(c *Config) {
			c.Environment = "production"
			c.Auth.CookieSecure = true
			c.Server.AllowedOrigins = "https://app.optikk.dev"
		}, true},
		{"production resend disabled skips email config requirement", func(c *Config) {
			c.Environment = "production"
			c.Auth.CookieSecure = true
			c.Server.AllowedOrigins = "https://app.optikk.dev"
			c.Email.ResendVerificationEnabled = false
		}, false},
		{"production resend enabled with email config", func(c *Config) {
			c.Environment = "production"
			c.Auth.CookieSecure = true
			c.Server.AllowedOrigins = "https://app.optikk.dev"
			c.Email.ResendAPIKey = "re_123"
			c.Email.From = "Optikk <noreply@optikk.dev>"
			c.Email.VerifyBaseURL = "https://app.optikk.dev/login"
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			tc.mutate(&c)
			err := c.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestEmailDefaults(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	if !v.GetBool("email.resend_verification_enabled") {
		t.Fatal("email.resend_verification_enabled should default to true")
	}
}

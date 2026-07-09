package config

import (
	"strings"
	"testing"
)

func validConfig() Config {
	var c Config
	c.Auth.JWTSecret = strings.Repeat("x", minJWTSecretLen)
	c.MySQL.Password = "pw"
	c.ClickHouse.Password = "pw"
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

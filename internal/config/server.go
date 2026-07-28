package config

import (
	"strings"
	"time"
)

type AlertingConfig struct {
	Kafka AlertingKafkaConfig `yaml:"kafka"`
}

type AlertingKafkaConfig struct {
	Enabled        bool     `yaml:"enabled"`
	BrokerList     string   `yaml:"broker_list"`
	BrokersList    []string `yaml:"brokers"`
	TopicPrefix    string   `yaml:"topic_prefix"`
	ConsumerGroup  string   `yaml:"consumer_group"`
	MaxPollRecords int      `yaml:"max_poll_records"`
}

func (c AlertingKafkaConfig) Brokers() []string {
	if c.BrokerList != "" {
		return strings.Split(c.BrokerList, ",")
	}
	return c.BrokersList
}

func (c AlertingKafkaConfig) MetricsTopic() string {
	prefix := c.TopicPrefix
	if prefix == "" {
		prefix = "optikk.ingest"
	}
	return prefix + ".metrics"
}

type ServerConfig struct {
	Port                      string `yaml:"port"`
	MetricsPort               string `yaml:"metrics_port"`
	AllowedOrigins            string `yaml:"allowed_origins"`
	DebugAPILogs              bool   `yaml:"debug_api_logs"`
	ExpensiveQueryConcurrency int    `yaml:"expensive_query_concurrency"`
}

func (c Config) ExpensiveQueryConcurrency() int {
	if n := c.Server.ExpensiveQueryConcurrency; n > 0 {
		return n
	}
	return 2
}

func (c Config) MetricsPort() string {
	if c.Server.MetricsPort != "" {
		return c.Server.MetricsPort
	}
	return "19091"
}

type AuthConfig struct {
	JWTSecret         string `yaml:"jwt_secret"`
	AccessTTLMs       int64  `yaml:"access_ttl_ms"`
	RefreshTTLMs      int64  `yaml:"refresh_ttl_ms"`
	RefreshCookieName string `yaml:"refresh_cookie_name"`
	CookieDomain      string `yaml:"cookie_domain"`
	CookieSecure      bool   `yaml:"cookie_secure"`
	CookieSameSite    string `yaml:"cookie_same_site"`
}

type LLMConfig struct {
	KeyEncryptionKey string `yaml:"key_encryption_key"`
}

type EmailConfig struct {
	ResendVerificationEnabled bool   `yaml:"resend_verification_enabled"`
	ResendAPIKey              string `yaml:"resend_api_key"`
	From                      string `yaml:"from"`
	VerifyBaseURL             string `yaml:"verify_base_url"`
}

func (c Config) AccessTokenTTL() time.Duration {
	return time.Duration(c.Auth.AccessTTLMs) * time.Millisecond
}

func (c Config) RefreshTokenTTL() time.Duration {
	return time.Duration(c.Auth.RefreshTTLMs) * time.Millisecond
}

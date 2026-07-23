package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

type Config struct {
	Environment string           `yaml:"environment"`
	Server      ServerConfig     `yaml:"server"`
	MySQL       MySQLConfig      `yaml:"mysql"`
	ClickHouse  ClickHouseConfig `yaml:"clickhouse"`
	Alerting    AlertingConfig   `yaml:"alerting"`
	Auth        AuthConfig       `yaml:"auth"`
	Email       EmailConfig      `yaml:"email"`
	LLM         LLMConfig        `yaml:"llm"`
}

// Load reads YAML configuration with environment variable overrides.
// If no path is provided, it defaults to "config.yml".
func Load(path ...string) (Config, error) {
	p := "config.yml"
	if len(path) > 0 && path[0] != "" {
		p = path[0]
	}

	resolved, err := resolveConfigFilePath(p)
	if err != nil {
		return Config{}, err
	}

	v := viper.New()
	v.SetConfigFile(resolved)

	setDefaults(v)

	v.SetEnvPrefix("OPTIKK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("cannot read config file %s: %w", resolved, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg, viper.DecoderConfigOption(func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "yaml"
	})); err != nil {
		return Config{}, fmt.Errorf("invalid config in %s: %w", resolved, err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// minJWTSecretLen guards against weak HMAC keys for HS256 access tokens.
const minJWTSecretLen = 32

// Validate rejects a config missing the secrets required to run safely.
// The service is always deployed in production, so these checks are
// unconditional. Each failing field is named so the startup log is actionable.
func (c Config) Validate() error {
	if len(c.Auth.JWTSecret) < minJWTSecretLen {
		return fmt.Errorf("auth.jwt_secret must be at least %d bytes", minJWTSecretLen)
	}
	if c.MySQL.Password == "" {
		return errors.New("mysql.password must not be empty")
	}
	if c.ClickHouse.Password == "" {
		return errors.New("clickhouse.password must not be empty")
	}
	if c.Alerting.Kafka.Enabled && len(c.Alerting.Kafka.Brokers()) == 0 {
		return errors.New("alerting.kafka.brokers must not be empty when alerting.kafka.enabled is true")
	}
	if c.Environment == "production" {
		if !c.Auth.CookieSecure {
			return errors.New("auth.cookie_secure must be true in production")
		}
		if c.Server.AllowedOrigins == "" || strings.Contains(c.Server.AllowedOrigins, "localhost") || strings.Contains(c.Server.AllowedOrigins, "*") {
			return errors.New("server.allowed_origins must be an explicit non-local production allowlist")
		}
		if c.Email.ResendVerificationEnabled && (c.Email.ResendAPIKey == "" || c.Email.From == "" || c.Email.VerifyBaseURL == "") {
			return errors.New("email configuration must be set in production")
		}
	}
	return nil
}

func resolveConfigFilePath(p string) (string, error) {
	if filepath.IsAbs(p) {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("config file %q: %w", p, err)
		}
		return p, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for dir := wd; ; {
		candidate := filepath.Join(dir, p)
		if st, statErr := os.Stat(candidate); statErr == nil && !st.IsDir() {
			return candidate, nil
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", fmt.Errorf("config file %q: %w", candidate, statErr)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("config file %q not found (searched from %s upward)", p, wd)
		}
		dir = parent
	}
}

func setDefaults(v *viper.Viper) {

	v.SetDefault("environment", "")

	v.SetDefault("server.port", "")
	v.SetDefault("server.allowed_origins", "")
	v.SetDefault("server.debug_api_logs", false)
	v.SetDefault("server.expensive_query_concurrency", 2)

	v.SetDefault("mysql.host", "")
	v.SetDefault("mysql.port", "")
	v.SetDefault("mysql.database", "")
	v.SetDefault("mysql.user", "")
	v.SetDefault("mysql.password", "")
	v.SetDefault("mysql.max_open_conns", 0)
	v.SetDefault("mysql.max_idle_conns", 0)

	v.SetDefault("clickhouse.host", "")
	v.SetDefault("clickhouse.port", "")
	v.SetDefault("clickhouse.database", "")
	v.SetDefault("clickhouse.user", "")
	v.SetDefault("clickhouse.password", "")
	v.SetDefault("clickhouse.production", false)
	v.SetDefault("clickhouse.cloud_host", "")
	v.SetDefault("clickhouse.max_open_conns", 0)
	v.SetDefault("clickhouse.max_idle_conns", 0)
	v.SetDefault("alerting.kafka.enabled", false)
	v.SetDefault("alerting.kafka.broker_list", "")
	v.SetDefault("alerting.kafka.topic_prefix", "optikk.ingest")
	v.SetDefault("alerting.kafka.consumer_group", "optikk-query-alerting")
	v.SetDefault("alerting.kafka.max_poll_records", 1000)

	v.SetDefault("auth.jwt_secret", "")
	v.SetDefault("auth.access_ttl_ms", 900000)
	v.SetDefault("auth.refresh_ttl_ms", 604800000)
	v.SetDefault("auth.refresh_cookie_name", "optikk_refresh")
	v.SetDefault("auth.cookie_domain", "")
	v.SetDefault("auth.cookie_secure", false)
	v.SetDefault("auth.cookie_same_site", "lax")
	v.SetDefault("email.resend_verification_enabled", true)
	v.SetDefault("email.resend_api_key", "")
	v.SetDefault("email.from", "")
	v.SetDefault("email.verify_base_url", "")

	v.SetDefault("llm.key_encryption_key", "")

}

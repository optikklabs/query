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
	Auth        AuthConfig       `yaml:"auth"`
	Email       EmailConfig      `yaml:"email"`
	LLM         LLMConfig        `yaml:"llm"`
	Ingestion   IngestionConfig  `yaml:"ingestion"`
	Billing     BillingConfig    `yaml:"billing"`
}

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

const minJWTSecretLen = 32

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
	v.SetDefault("server.metrics_port", "19091")
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
	v.SetDefault("clickhouse.max_open_conns", 0)
	v.SetDefault("clickhouse.max_idle_conns", 0)

	v.SetDefault("clickhouse.query_budgets.dashboard.max_execution_time", 10)
	v.SetDefault("clickhouse.query_budgets.dashboard.max_rows_to_read", 300_000_000)
	v.SetDefault("clickhouse.query_budgets.dashboard.max_memory_usage", 2*1024*1024*1024)
	v.SetDefault("clickhouse.query_budgets.dashboard.max_result_rows", 100_000)
	v.SetDefault("clickhouse.query_budgets.dashboard.max_threads", 4)
	v.SetDefault("clickhouse.query_budgets.dashboard.priority", 1)

	v.SetDefault("clickhouse.query_budgets.overview.max_execution_time", 30)
	v.SetDefault("clickhouse.query_budgets.overview.max_rows_to_read", 500_000_000)
	v.SetDefault("clickhouse.query_budgets.overview.max_memory_usage", 4*1024*1024*1024)
	v.SetDefault("clickhouse.query_budgets.overview.max_result_rows", 100_000)
	v.SetDefault("clickhouse.query_budgets.overview.max_threads", 4)
	v.SetDefault("clickhouse.query_budgets.overview.priority", 5)

	v.SetDefault("clickhouse.query_budgets.explorer.max_execution_time", 60)
	v.SetDefault("clickhouse.query_budgets.explorer.max_rows_to_read", 1_000_000_000)
	v.SetDefault("clickhouse.query_budgets.explorer.max_memory_usage", 4*1024*1024*1024)
	v.SetDefault("clickhouse.query_budgets.explorer.max_result_rows", 100_000)
	v.SetDefault("clickhouse.query_budgets.explorer.max_threads", 4)
	v.SetDefault("clickhouse.query_budgets.explorer.priority", 10)
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

	v.SetDefault("ingestion.public_grpc_endpoint", "ingest.optikk.in:4317")
	v.SetDefault("ingestion.public_http_endpoint", "https://ingest.optikk.in:4318")

	v.SetDefault("billing.gb_price_usd", 0.10)
	v.SetDefault("billing.dpm_price_usd", 0.008)
	v.SetDefault("billing.monthly_record_commitment", 5_000_000_000)

}

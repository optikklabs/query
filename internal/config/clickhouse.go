package config

import (
	"crypto/tls"
	"net"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type ClickHouseConfig struct {
	Host         string             `yaml:"host"`
	Port         string             `yaml:"port"`
	Database     string             `yaml:"database"`
	User         string             `yaml:"user"`
	Password     string             `yaml:"password"`
	Secure       bool               `yaml:"secure"`
	MaxOpenConns int                `yaml:"max_open_conns"`
	MaxIdleConns int                `yaml:"max_idle_conns"`
	QueryBudgets QueryBudgetsConfig `yaml:"query_budgets"`
}

type QueryBudgetsConfig struct {
	Dashboard QueryBudget `yaml:"dashboard"`
	Overview  QueryBudget `yaml:"overview"`
	Explorer  QueryBudget `yaml:"explorer"`
}

type QueryBudget struct {
	MaxExecutionTime int   `yaml:"max_execution_time"`
	MaxRowsToRead    int64 `yaml:"max_rows_to_read"`
	MaxMemoryUsage   int64 `yaml:"max_memory_usage"`
	MaxResultRows    int64 `yaml:"max_result_rows"`
	MaxThreads       int   `yaml:"max_threads"`
	Priority         int   `yaml:"priority"`
}

// ClickHouseOptions builds the client settings for the configured server.
func (c Config) ClickHouseOptions() *clickhouse.Options {
	opts := &clickhouse.Options{
		Addr: []string{net.JoinHostPort(c.ClickHouse.Host, c.ClickHouse.Port)},
		Auth: clickhouse.Auth{
			Database: c.ClickHouse.Database,
			Username: c.ClickHouse.User,
			Password: c.ClickHouse.Password,
		},
		MaxOpenConns: c.ClickHouse.MaxOpenConns,
		MaxIdleConns: c.ClickHouse.MaxIdleConns,
	}
	if c.ClickHouse.Secure {
		opts.TLS = &tls.Config{}
	}
	return opts
}

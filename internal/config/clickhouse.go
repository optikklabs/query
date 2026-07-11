package config

import "fmt"

type ClickHouseConfig struct {
	Host         string `yaml:"host"`
	Port         string `yaml:"port"`
	Database     string `yaml:"database"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	Secure       bool   `yaml:"secure"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

// Pool defaults are sized so all query replicas together (HPA max 5 × 40)
// stay near ClickHouse's default max_concurrent_queries (100) with headroom;
// brief queuing in the Go pool beats server-side rejections.
func (c Config) ClickHouseMaxOpenConns() int {
	if n := c.ClickHouse.MaxOpenConns; n > 0 {
		return n
	}
	return 40
}

func (c Config) ClickHouseMaxIdleConns() int {
	if n := c.ClickHouse.MaxIdleConns; n > 0 {
		return n
	}
	return 20
}

func (c Config) ClickHouseDSN() string {
	dsn := fmt.Sprintf("clickhouse://%s:%s@%s:%s/%s",
		c.ClickHouse.User,
		c.ClickHouse.Password,
		c.ClickHouse.Host,
		c.ClickHouse.Port,
		c.ClickHouse.Database,
	)
	if c.ClickHouse.Secure {
		dsn += "?secure=true"
	}
	return dsn
}

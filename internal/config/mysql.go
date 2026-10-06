package config

import (
	"net"
	"time"

	"github.com/go-sql-driver/mysql"
)

type MySQLConfig struct {
	Host         string `yaml:"host"`
	Port         string `yaml:"port"`
	Database     string `yaml:"database"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

// MySQLDriverConfig builds the driver connection settings. ClientFoundRows
// makes RowsAffected count matched rows, so an UPDATE that rewrites identical
// values is not mistaken for a missing row. Sessions run in UTC.
func (c Config) MySQLDriverConfig() (*mysql.Config, error) {
	cfg := mysql.NewConfig()
	cfg.User = c.MySQL.User
	cfg.Passwd = c.MySQL.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(c.MySQL.Host, c.MySQL.Port)
	cfg.DBName = c.MySQL.Database
	cfg.ParseTime = true
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 30 * time.Second
	cfg.WriteTimeout = 30 * time.Second
	cfg.ClientFoundRows = true
	// NOW() and CURRENT_TIMESTAMP defaults must agree with the UTC times the
	// app writes, whatever the server's timezone.
	cfg.Params = map[string]string{"time_zone": "'+00:00'"}
	if err := cfg.Apply(mysql.Charset("utf8mb4", "")); err != nil {
		return nil, err
	}
	return cfg, nil
}

package app

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/optikklabs/query/internal/config"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/secretbox"
	"github.com/optikklabs/query/internal/infra/token"
)

type Infra struct {
	Config    config.Config
	DB        *sql.DB
	CH        clickhouse.Conn
	Tokens    *token.Service
	SecretBox *secretbox.Box
}

func newInfra(cfg config.Config) (_ *Infra, err error) {
	box, err := secretbox.New(cfg.LLM.KeyEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("llm.key_encryption_key: %w", err)
	}

	dbConn, err := openMySQL(cfg)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = dbConn.Close()
		}
	}()

	chConn, err := openClickHouse(cfg)
	if err != nil {
		return nil, err
	}

	return &Infra{
		Config:    cfg,
		DB:        dbConn,
		CH:        chConn,
		Tokens:    token.NewService(cfg),
		SecretBox: box,
	}, nil
}

func openMySQL(cfg config.Config) (*sql.DB, error) {
	driverCfg, err := cfg.MySQLDriverConfig()
	if err != nil {
		return nil, fmt.Errorf("mysql: %w", err)
	}
	dbConn, err := dbutil.Open(driverCfg, cfg.MySQL.MaxOpenConns, cfg.MySQL.MaxIdleConns)
	if err != nil {
		return nil, fmt.Errorf("mysql: %w", err)
	}
	slog.Info("mysql connected",
		slog.String("addr", net.JoinHostPort(cfg.MySQL.Host, cfg.MySQL.Port)),
		slog.String("database", cfg.MySQL.Database),
		slog.Int("max_open_conns", cfg.MySQL.MaxOpenConns),
	)
	return dbConn, nil
}

func openClickHouse(cfg config.Config) (clickhouse.Conn, error) {
	dbutil.InitQueryBudgets(cfg.ClickHouse.QueryBudgets)
	chConn, err := dbutil.OpenClickHouseConn(cfg.ClickHouseOptions())
	if err != nil {
		return nil, fmt.Errorf("clickhouse: %w", err)
	}
	slog.Info("clickhouse connected",
		slog.String("addr", net.JoinHostPort(cfg.ClickHouse.Host, cfg.ClickHouse.Port)),
		slog.String("database", cfg.ClickHouse.Database),
	)
	return chConn, nil
}

func (i *Infra) Close() error {
	return errors.Join(i.CH.Close(), i.DB.Close())
}

package onboarding

import (
	"context"
	"database/sql"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/jmoiron/sqlx"

	dbutil "github.com/optikklabs/query/internal/infra/database"
)

// TeamStatus is the provisioning row read from MySQL.
type TeamStatus struct {
	Slug   string `db:"slug"`
	APIKey string `db:"api_key"`
	Status string `db:"provisioning_status"`
}

// Repository reads team provisioning state (MySQL) and first-data marks (ClickHouse).
type Repository struct {
	db *sqlx.DB
	ch clickhouse.Conn
}

func NewRepository(db *sql.DB, ch clickhouse.Conn) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql"), ch: ch}
}

func (r *Repository) TeamStatus(ctx context.Context, teamID int64) (TeamStatus, error) {
	var t TeamStatus
	err := dbutil.GetSQL(ctx, r.db, "onboarding.TeamStatus", &t, `
		SELECT slug, api_key, provisioning_status
		FROM teams
		WHERE id = ? AND active = 1
		LIMIT 1
	`, teamID)
	return t, err
}

// FirstSeen returns the team's earliest event in table, or nil if none landed.
// table is a compile-time constant ("spans", "logs", "metrics"), never user input.
func (r *Repository) FirstSeen(ctx context.Context, table string, teamID int64) (*time.Time, error) {
	var rows []struct {
		Count uint64    `ch:"c"`
		First time.Time `ch:"t"`
	}
	err := dbutil.SelectCH(ctx, r.ch, "onboarding.FirstSeen."+table, &rows, `
		SELECT count() AS c, min(timestamp) AS t
		FROM optikk.`+table+`
		PREWHERE team_id = @teamID
	`, clickhouse.Named("teamID", uint32(teamID)))
	if err != nil || len(rows) == 0 || rows[0].Count == 0 {
		return nil, err
	}
	first := rows[0].First.UTC()
	return &first, nil
}

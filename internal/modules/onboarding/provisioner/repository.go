package provisioner

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"

	dbutil "github.com/optikklabs/query/internal/infra/database"
)

// PendingTeam is a team awaiting collector provisioning.
type PendingTeam struct {
	ID     int64  `db:"id"`
	Slug   string `db:"slug"`
	APIKey string `db:"api_key"`
}

// Repository reads and updates teams.provisioning_status in MySQL.
type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql")}
}

func (r *Repository) ListPending(ctx context.Context) ([]PendingTeam, error) {
	var teams []PendingTeam
	err := dbutil.SelectSQL(ctx, r.db, "provisioner.ListPending", &teams, `
		SELECT id, slug, api_key
		FROM teams
		WHERE provisioning_status = 'pending' AND active = 1
		ORDER BY id
	`)
	return teams, err
}

func (r *Repository) SetStatus(ctx context.Context, teamID int64, status string) error {
	_, err := dbutil.ExecSQL(ctx, r.db, "provisioner.SetStatus", `
		UPDATE teams SET provisioning_status = ? WHERE id = ?
	`, status, teamID)
	return err
}

// Package billing runs the BackgroundRunner that enforces trial lifecycle.
package billing

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
	dbutil "github.com/optikklabs/query/internal/infra/database"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql")}
}

// SuspendExpiredTrials moves trialing tenants past their deadline to suspended
// and flips active off, cutting ingest and login. Returns rows affected.
func (r *Repository) SuspendExpiredTrials(ctx context.Context, now time.Time) (int64, error) {
	res, err := dbutil.ExecSQL(ctx, r.db, "billing.SuspendExpiredTrials", `
		UPDATE optikk.tenant
		   SET account_status = 'suspended', active = 0
		 WHERE account_status = 'trialing' AND trial_ends_at < ?
	`, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

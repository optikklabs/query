package providerkeys

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
	dbutil "github.com/optikklabs/query/internal/infra/database"
)

// Repository owns MySQL persistence for encrypted provider keys.
type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: sqlx.NewDb(db, "mysql")}
}

func (r *Repository) List(ctx context.Context, tenantID int64) ([]keyRow, error) {
	var rows []keyRow
	err := dbutil.SelectSQL(ctx, r.db, "providerkeys.List", &rows,
		`SELECT id, provider, label, key_last4, created_at
		   FROM optikk.llm_provider_keys WHERE tenant_id = ? ORDER BY provider, label`, tenantID)
	return rows, err
}

type insertArgs struct {
	TenantID   int64
	Provider   string
	Label      string
	Ciphertext []byte
	Nonce      []byte
	Last4      string
	CreatedBy  sql.NullInt64
}

func (r *Repository) Create(ctx context.Context, a insertArgs) (int64, error) {
	res, err := dbutil.ExecSQL(ctx, r.db, "providerkeys.Create", `
		INSERT INTO optikk.llm_provider_keys
		  (tenant_id, provider, label, key_ciphertext, nonce, key_last4, created_at, created_by_user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.TenantID, a.Provider, a.Label, a.Ciphertext, a.Nonce, a.Last4, time.Now().UTC(), a.CreatedBy)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repository) Get(ctx context.Context, tenantID, id int64) (keyRow, error) {
	var row keyRow
	err := dbutil.GetSQL(ctx, r.db, "providerkeys.Get", &row,
		`SELECT id, provider, label, key_last4, created_at
		   FROM optikk.llm_provider_keys WHERE tenant_id = ? AND id = ? LIMIT 1`, tenantID, id)
	return row, err
}

func (r *Repository) Delete(ctx context.Context, tenantID, id int64) error {
	res, err := dbutil.ExecSQL(ctx, r.db, "providerkeys.Delete",
		`DELETE FROM optikk.llm_provider_keys WHERE tenant_id = ? AND id = ?`, tenantID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Secret fetches the ciphertext for the tenant's key of a provider, preferring
// the most recently created. Used only for outbound provider calls.
func (r *Repository) Secret(ctx context.Context, tenantID int64, provider string) (secretRow, error) {
	var row secretRow
	err := dbutil.GetSQL(ctx, r.db, "providerkeys.Secret", &row,
		`SELECT key_ciphertext, nonce FROM optikk.llm_provider_keys
		  WHERE tenant_id = ? AND provider = ? ORDER BY created_at DESC LIMIT 1`, tenantID, provider)
	return row, err
}

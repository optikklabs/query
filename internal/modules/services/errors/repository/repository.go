package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/services/errors/models"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// serviceClause returns an optional AND-clause plus its argument for queries
// that filter to a single service when serviceName is non-empty.
func serviceClause(serviceName string) (string, []any) {
	if serviceName == "" {
		return "", nil
	}
	return `
		     AND service = @serviceName`, []any{clickhouse.Named("serviceName", serviceName)}
}

func (r *Repository) ServiceErrorRateRows(ctx context.Context, tenantID int64, startMs, endMs int64, serviceName string) ([]models.RawServiceRateRow, error) {
	clause, clauseArgs := serviceClause(serviceName)
	query := `
		SELECT service                                           AS service_name,
		       ` + timebucket.DisplayGrainSQLForRange(startMs, endMs) + ` AS bucket_at,
		       ` + spanstats.Requests + `,
		       ` + spanstats.Errors + `,
		       ` + spanstats.DurationSum + `
		FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + `
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end` + clause + `
		GROUP BY service_name, bucket_at
		ORDER BY bucket_at ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), clauseArgs...)
	var rows []models.RawServiceRateRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "errors.ServiceErrorRate", &rows, query, args...)
}

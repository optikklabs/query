package log_facets //nolint:revive,stylecheck

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/logs/filter"
)

const facetTopN = 50

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository { return &Repository{db: db} }

// dimRow is the scan target for the unified facets query.
type dimRow struct {
	Dim   string `ch:"dim"`
	Value string `ch:"value"`
	Count uint64 `ch:"cnt"`
}

func (r *Repository) Compute(ctx context.Context, f filter.Filters) ([]dimRow, error) {
	resourceWhere, _, args := filter.BuildClauses(f)
	args = append(args, clickhouse.Named("facetLimit", uint64(facetTopN)))

	query := facetArm("service", resourceWhere) +
		" UNION ALL " + facetArm("host", resourceWhere) +
		" UNION ALL " + facetArm("pod", resourceWhere) +
		" UNION ALL " + facetArm("environment", resourceWhere)

	var rows []dimRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "logsFacets.Compute",
		&rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func facetArm(dim, resourceWhere string) string {
	return `
		SELECT '` + dim + `' AS dim, ` + dim + ` AS value, count() AS cnt
		FROM optikk.logs_resource
		PREWHERE team_id = @teamID` + resourceWhere + `
		WHERE ` + dim + ` != ''
		GROUP BY ` + dim + `
		ORDER BY cnt DESC
		LIMIT @facetLimit`
}

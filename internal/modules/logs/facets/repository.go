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

	// One UNION ALL arm per facet dimension; querying logs_resource directly.
	arm := func(dim string) string {
		return `
			SELECT '` + dim + `' AS dim, ` + dim + ` AS value, count() AS cnt
			FROM optikk.logs_resource
			PREWHERE tenant_id = @tenantID` + resourceWhere + `
			WHERE ` + dim + ` != ''
			GROUP BY ` + dim + `
			ORDER BY cnt DESC
			LIMIT @facetLimit`
	}

	query := arm("service") +
		" UNION ALL " + arm("host") +
		" UNION ALL " + arm("pod") +
		" UNION ALL " + arm("environment")

	var rows []dimRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "logsFacets.Compute",
		&rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

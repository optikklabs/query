package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/logs/filter"
)

const facetTopN = 50

type DimRow struct {
	Dim   string `ch:"dim"`
	Value string `ch:"value"`
	Count uint64 `ch:"cnt"`
}

func buildFacetQuery(source string) string {
	return `SELECT
			multiIf(
				grouping(service) = 0, 'service',
				grouping(host) = 0, 'host',
				grouping(pod) = 0, 'pod',
				grouping(environment) = 0, 'environment',
				''
			) AS dim,
			multiIf(
				grouping(service) = 0, service,
				grouping(host) = 0, host,
				grouping(pod) = 0, pod,
				grouping(environment) = 0, environment,
				''
			) AS value,
			sum(log_count) AS cnt
		FROM ` + source + `
		GROUP BY GROUPING SETS (
			(service),
			(host),
			(pod),
			(environment)
		)
		HAVING value != ''
		ORDER BY dim, cnt DESC, value ASC
		LIMIT @facetLimit BY dim`
}

func (r *Repository) Facets(ctx context.Context, f filter.Filters) ([]DimRow, error) {
	const columns = "service, host, pod, environment"
	source, args := aggregateSource(f, columns, columns, columns)
	args = append(args, clickhouse.Named("facetLimit", uint64(facetTopN)))
	query := buildFacetQuery(source)

	var rows []DimRow
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "logsFacets.Compute",
		&rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

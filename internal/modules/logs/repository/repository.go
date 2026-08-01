package repository

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/modules/logs/filter"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository { return &Repository{db: db} }

// aggregateSource returns one weighted row set for aggregate endpoints. The
// raw-only plan and the rollup-plus-boundaries plan share every outer query.
func aggregateSource(f filter.Filters, rawColumns, rollupColumns, groupBy string) (string, []any) {
	if c, ok := filter.BuildStatsClauses(f); ok {
		return `(SELECT ` + rawColumns + `, count() AS log_count
			FROM optikk.logs ` + c.RawPrewhere + ` ` + c.Where + ` GROUP BY ` + groupBy + `
			UNION ALL
			SELECT ` + rollupColumns + `, sum(log_count) AS log_count
			FROM optikk.logs_stats_1m ` + c.RollupPrewhere + ` ` + c.Where + ` GROUP BY ` + groupBy + `)`, c.Args
	}
	prewhere, where, args := filter.BuildClauses(f)
	return `(SELECT ` + rawColumns + `, count() AS log_count
		FROM optikk.logs ` + prewhere + ` ` + where + ` GROUP BY ` + groupBy + `)`, args
}

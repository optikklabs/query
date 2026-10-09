package filter

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/filterutil"
)

const MetricDBSQLConnectionOpen = "db.sql.connection.open"

type Filters struct {
	DBSystem   []string
	Collection []string
}

// ExplorerFilters contains predicates supported by the normalized-query
// explorer. Row-level fields are applied before grouping; aggregate bounds are
// applied in HAVING so pagination and filtering remain complete.
type ExplorerFilters struct {
	DBSystems   []string `json:"dbSystems,omitempty"`
	Collections []string `json:"collections,omitempty"`
	Services    []string `json:"services,omitempty"`
	QueryText   string   `json:"queryText,omitempty"`

	MinCallCount  *uint64  `json:"minCallCount,omitempty"`
	MaxCallCount  *uint64  `json:"maxCallCount,omitempty"`
	MinErrorCount *uint64  `json:"minErrorCount,omitempty"`
	MaxErrorCount *uint64  `json:"maxErrorCount,omitempty"`
	MinP50Ms      *float64 `json:"minP50Ms,omitempty"`
	MaxP50Ms      *float64 `json:"maxP50Ms,omitempty"`
	MinP95Ms      *float64 `json:"minP95Ms,omitempty"`
	MaxP95Ms      *float64 `json:"maxP95Ms,omitempty"`
	MinP99Ms      *float64 `json:"minP99Ms,omitempty"`
	MaxP99Ms      *float64 `json:"maxP99Ms,omitempty"`
}

func ParseFilters(r *http.Request) Filters {
	return Filters{
		DBSystem:   r.URL.Query()["dbSystem"],
		Collection: r.URL.Query()["collection"],
	}
}

// BuildSpanClauses filters optikk.spans and the span_stats rollups alike.
func BuildSpanClauses(f Filters) (where string, args []any) {
	args = filterutil.AppendIn(&where, args,
		filterutil.InClause{Column: "db_system", Bind: "dbSystem", Values: f.DBSystem},
		filterutil.InClause{Column: "db_name", Bind: "dbCollection", Values: f.Collection},
	)
	return where, args
}

func BuildMetricsClauses(f Filters) (where string, args []any) {
	args = filterutil.AppendIn(&where, args,
		filterutil.InClause{Column: "db_system", Bind: "dbSystem", Values: f.DBSystem},
	)
	return where, args
}

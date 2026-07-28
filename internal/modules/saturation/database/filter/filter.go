package filter

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/filterutil"
)

const (
	AttrDBNamespace   = "db.namespace"
	AttrServerAddress = "server.address"
)

const MetricDBSQLConnectionOpen = "db.sql.connection.open"

type Filters struct {
	DBSystem   []string
	Collection []string
	Namespace  []string
	Server     []string
}

func ParseFilters(r *http.Request) Filters {
	return Filters{
		DBSystem:   r.URL.Query()["dbSystem"],
		Collection: r.URL.Query()["collection"],
		Namespace:  r.URL.Query()["namespace"],
		Server:     r.URL.Query()["server"],
	}
}

func BuildSpanClauses(f Filters) (where string, args []any) {
	args = filterutil.AppendIn(&where, args,
		filterutil.InClause{Column: "db_system", Bind: "dbSystem", Values: f.DBSystem},
		filterutil.InClause{Column: "db_name", Bind: "dbCollection", Values: f.Collection},
		filterutil.InClause{Column: `attributes['` + AttrDBNamespace + `']`,
			Bind: "dbNamespace", Values: f.Namespace},
		filterutil.InClause{Column: `attributes['` + AttrServerAddress + `']`,
			Bind: "dbServer", Values: f.Server},
	)
	return where, args
}

func BuildMetricsClauses(f Filters) (where string, args []any) {
	args = filterutil.AppendIn(&where, args,
		filterutil.InClause{Column: "db_system", Bind: "dbSystem", Values: f.DBSystem},
	)
	return where, args
}

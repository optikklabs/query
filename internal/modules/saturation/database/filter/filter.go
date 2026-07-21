package filter

import (
	"net/http"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// OTel semantic-convention names and canonical metric names.

const (
	AttrDBSystem      = "db.system"
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
	if len(f.DBSystem) > 0 {
		where += ` AND db_system IN @dbSystem`
		args = append(args, clickhouse.Named("dbSystem", f.DBSystem))
	}
	if len(f.Collection) > 0 {
		where += ` AND db_name IN @dbCollection`
		args = append(args, clickhouse.Named("dbCollection", f.Collection))
	}
	if len(f.Namespace) > 0 {
		where += ` AND attributes[` + AttrDBNamespace + `] IN @dbNamespace`
		args = append(args, clickhouse.Named("dbNamespace", f.Namespace))
	}
	if len(f.Server) > 0 {
		where += ` AND attributes[` + AttrServerAddress + `] IN @dbServer`
		args = append(args, clickhouse.Named("dbServer", f.Server))
	}
	return where, args
}

// BuildMetricsClauses only filters dimensions written by the span-metrics
// producer. Collection, namespace, and server remain raw-span filters.
func BuildMetricsClauses(f Filters) (where string, args []any) {
	if len(f.DBSystem) > 0 {
		where += " AND attributes['db.system'] IN @dbSystem"
		args = append(args, clickhouse.Named("dbSystem", f.DBSystem))
	}
	return where, args
}

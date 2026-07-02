package filter

import (
	"strconv"
	"time"

	"net/http"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// OTel semantic-convention names and canonical metric names.

const (
	AttrDBSystem         = "db.system"
	AttrDBNamespace      = "db.namespace"
	AttrDBOperationName  = "db.operation.name"
	AttrDBCollectionName = "db.collection.name"
	AttrDBQueryText      = "db.query.text"
	AttrDBResponseStatus = "db.response.status_code"
	AttrErrorType        = "error.type"
	AttrServerAddress    = "server.address"
	AttrServerPort       = "server.port"
	AttrPoolName         = "pool.name"
	AttrConnectionState  = "db.client.connection.state"
)

const (
	MetricDBOperationDuration = "db.client.operation.duration"

	MetricDBSQLConnectionOpen = "db.sql.connection.open"

	MetricDBConnectionCount      = "db.client.connection.count"
	MetricDBConnectionMax        = "db.client.connection.max"
	MetricDBConnectionIdleMax    = "db.client.connection.idle.max"
	MetricDBConnectionIdleMin    = "db.client.connection.idle.min"
	MetricDBConnectionPendReqs   = "db.client.connection.pending_requests"
	MetricDBConnectionTimeouts   = "db.client.connection.timeouts"
	MetricDBConnectionCreateTime = "db.client.connection.create_time"
	MetricDBConnectionWaitTime   = "db.client.connection.wait_time"
	MetricDBConnectionUseTime    = "db.client.connection.use_time"
)

type Filters struct {
	DBSystem   []string
	Collection []string
	Namespace  []string
	Server     []string
}

func ParseFilters(r *http.Request) Filters {
	return Filters{
		DBSystem:   r.URL.Query()["db_system"],
		Collection: r.URL.Query()["collection"],
		Namespace:  r.URL.Query()["namespace"],
		Server:     r.URL.Query()["server"],
	}
}

func ParseLimit(r *http.Request, def int) int {
	if s := r.URL.Query().Get("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			return v
		}
	}
	return def
}

func SpanArgs(teamID, startMs, endMs int64) []any {
	return []any{
		clickhouse.Named("teamID", uint32(teamID)),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
	}
}

func MetricArgs(teamID, startMs, endMs int64, metricName string) []any {
	return []any{
		clickhouse.Named("teamID", uint32(teamID)),
		clickhouse.Named("metricName", metricName),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
	}
}

func BuildSpanClauses(f Filters) (where string, args []any) {
	if len(f.DBSystem) > 0 {
		where += ` AND db_system IN @dbSystem`
		args = append(args, clickhouse.Named("dbSystem", f.DBSystem))
	}
	if len(f.Collection) > 0 {
		where += ` AND attributes.'` + AttrDBCollectionName + `'::String IN @dbCollection`
		args = append(args, clickhouse.Named("dbCollection", f.Collection))
	}
	if len(f.Namespace) > 0 {
		where += ` AND attributes.'` + AttrDBNamespace + `'::String IN @dbNamespace`
		args = append(args, clickhouse.Named("dbNamespace", f.Namespace))
	}
	if len(f.Server) > 0 {
		where += ` AND attributes.'` + AttrServerAddress + `'::String IN @dbServer`
		args = append(args, clickhouse.Named("dbServer", f.Server))
	}
	return where, args
}

func MetricsGroupColumn(attr string) string {
	switch attr {
	case AttrDBSystem:
		return "attributes.`db.system`::String"
	case AttrDBOperationName:
		return "attributes.`db.operation.name`::String"
	case AttrDBCollectionName:
		return "attributes.`db.collection.name`::String"
	case AttrDBNamespace:
		return "attributes.`db.namespace`::String"
	case AttrServerAddress:
		return "attributes.`server.address`::String"
	}
	return ""
}

func BuildMetricsClauses(f Filters) (where string, args []any) {
	if len(f.DBSystem) > 0 {
		where += " AND attributes.`db.system`::String IN @dbSystem"
		args = append(args, clickhouse.Named("dbSystem", f.DBSystem))
	}
	if len(f.Collection) > 0 {
		where += " AND attributes.`db.collection.name`::String IN @dbCollection"
		args = append(args, clickhouse.Named("dbCollection", f.Collection))
	}
	if len(f.Namespace) > 0 {
		where += " AND attributes.`db.namespace`::String IN @dbNamespace"
		args = append(args, clickhouse.Named("dbNamespace", f.Namespace))
	}
	if len(f.Server) > 0 {
		where += " AND attributes.`server.address`::String IN @dbServer"
		args = append(args, clickhouse.Named("dbServer", f.Server))
	}
	return where, args
}

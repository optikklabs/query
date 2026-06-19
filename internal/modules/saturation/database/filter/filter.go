package filter

import (
	"strconv"
	"time"

	"net/http"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/infra/timebucket"
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

	// MetricDBSQLConnectionOpen is the otelsql open-connection gauge actually
	// emitted by the stack (the db.client.connection.* semconv metrics are not).
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

// ParseFilters extracts query-string filters into the typed shape consumed
// by every repository.
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
	bucketStart, bucketEnd := SpanBucketBounds(startMs, endMs)
	return []any{
		clickhouse.Named("teamID", uint32(teamID)),
		clickhouse.Named("bucketStart", bucketStart),
		clickhouse.Named("bucketEnd", bucketEnd),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
	}
}

func SpanBucketBounds(startMs, endMs int64) (uint32, uint32) {
	return timebucket.BucketStart(startMs / 1000),
		timebucket.BucketStart(endMs/1000) + uint32(timebucket.BucketSeconds)
}

func MetricArgs(teamID, startMs, endMs int64, metricName string) []any {
	bucketStart, bucketEnd := MetricBucketBounds(startMs, endMs)
	return []any{
		clickhouse.Named("teamID", uint32(teamID)),
		clickhouse.Named("bucketStart", bucketStart),
		clickhouse.Named("bucketEnd", bucketEnd),
		clickhouse.Named("metricName", metricName),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
	}
}

func MetricBucketBounds(startMs, endMs int64) (uint32, uint32) {
	return timebucket.BucketStart(startMs / 1000),
		timebucket.BucketStart(endMs/1000) + uint32(timebucket.BucketSeconds)
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

// BuildSpans1mClauses builds SQL filter clauses for the spans_1m table.
func BuildSpans1mClauses(f Filters) (where string, args []any) {
	if len(f.DBSystem) > 0 {
		where += ` AND db_system IN @dbSystem`
		args = append(args, clickhouse.Named("dbSystem", f.DBSystem))
	}
	if len(f.Collection) > 0 {
		where += ` AND db_collection_name IN @dbCollection`
		args = append(args, clickhouse.Named("dbCollection", f.Collection))
	}
	if len(f.Namespace) > 0 {
		where += ` AND db_namespace IN @dbNamespace`
		args = append(args, clickhouse.Named("dbNamespace", f.Namespace))
	}
	if len(f.Server) > 0 {
		where += ` AND server_address IN @dbServer`
		args = append(args, clickhouse.Named("dbServer", f.Server))
	}
	return where, args
}

// Spans1mGroupColumn returns the spans_1m column name for the attribute.
func Spans1mGroupColumn(attr string) string {
	switch attr {
	case AttrDBSystem:
		return "db_system"
	case AttrDBOperationName:
		return "db_operation_name"
	case AttrDBCollectionName:
		return "db_collection_name"
	case AttrDBNamespace:
		return "db_namespace"
	case AttrServerAddress:
		return "server_address"
	case AttrErrorType:
		return "error_type"
	case AttrDBResponseStatus:
		return "db_response_status"
	}
	return ""
}

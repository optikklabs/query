package redfleet

import (
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/shared/chargs"
)

const seriesIndexBucket = 6 * time.Hour

// REDFilters captures the optional service filter for RED metric queries.
// When Services is empty, queries run fleet-wide (no filter applied).
// When Services contains one or more entries, ClickHouse PREWHERE limits
// fingerprint resolution to only matching services.
type REDFilters struct {
	TenantID int64
	StartMs  int64
	EndMs    int64
	Services []string
}

// BuildREDClauses returns a WHERE clause fragment and named ClickHouse args
// for metrics_series queries. The clause is safe to inject after "WHERE 1=1"
// in CTE series subqueries.
//
// Every clause constrains the six-hour expression in the metrics_series sort
// key. Optional service predicates are appended for narrower queries.
func BuildREDClauses(f REDFilters) (seriesWhere string, args []any) {
	args = chargs.RollupRangeArgs(f.TenantID, f.StartMs, f.EndMs)
	seriesWhere = `AND toStartOfInterval(s.timestamp, INTERVAL 6 HOUR)
		BETWEEN @seriesBucketStart AND @seriesBucketEnd`
	args = append(args,
		clickhouse.Named("seriesBucketStart", time.UnixMilli(f.StartMs).UTC().Truncate(seriesIndexBucket)),
		clickhouse.Named("seriesBucketEnd", time.UnixMilli(f.EndMs).UTC().Truncate(seriesIndexBucket)),
	)
	if len(f.Services) == 1 {
		seriesWhere += " AND s.service = @serviceName"
		args = append(args, clickhouse.Named("serviceName", f.Services[0]))
	} else if len(f.Services) > 1 {
		seriesWhere += " AND s.service IN @services"
		args = append(args, clickhouse.Named("services", f.Services))
	}
	return seriesWhere, args
}

// SingleService is a convenience that returns the single service name if
// exactly one service is in the filter, or "" otherwise.
func (f REDFilters) SingleService() string {
	if len(f.Services) == 1 {
		return f.Services[0]
	}
	return ""
}

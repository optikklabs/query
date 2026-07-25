package redfleet

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/shared/chargs"
)

// REDFilters captures the optional service filter for RED metric queries.
// When Services is empty, queries run fleet-wide (no filter applied).
type REDFilters struct {
	TenantID int64
	StartMs  int64
	EndMs    int64
	Services []string
}

// BuildREDClauses returns a predicate fragment and named ClickHouse args for
// span_stats queries. The clause is safe to append to a PREWHERE that already
// constrains tenant_id and timestamp.
func BuildREDClauses(f REDFilters) (where string, args []any) {
	args = chargs.RollupRangeArgs(f.TenantID, f.StartMs, f.EndMs)
	if len(f.Services) == 1 {
		where = " AND service = @serviceName"
		args = append(args, clickhouse.Named("serviceName", f.Services[0]))
	} else if len(f.Services) > 1 {
		where = " AND service IN @services"
		args = append(args, clickhouse.Named("services", f.Services))
	}
	return where, args
}

// SingleService is a convenience that returns the single service name if
// exactly one service is in the filter, or "" otherwise.
func (f REDFilters) SingleService() string {
	if len(f.Services) == 1 {
		return f.Services[0]
	}
	return ""
}

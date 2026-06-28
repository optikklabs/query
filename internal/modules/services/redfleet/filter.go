package redfleet

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/shared/chargs"
)

// REDFilters captures the optional service filter for RED metric queries.
// When Services is empty, queries run fleet-wide (no filter applied).
// When Services contains one or more entries, ClickHouse PREWHERE limits
// fingerprint resolution to only matching services.
type REDFilters struct {
	TeamID   int64
	StartMs  int64
	EndMs    int64
	Services []string
}

// BuildREDClauses returns a WHERE clause fragment and named ClickHouse args
// for metrics_series queries. The clause is safe to inject after "WHERE 1=1"
// in CTE series subqueries.
//
//   - empty Services  →  seriesWhere = ""          (fleet-wide)
//   - single Service  →  seriesWhere = "AND s.service = @serviceName"
//   - multi  Services →  seriesWhere = "AND s.service IN @services"
func BuildREDClauses(f REDFilters) (seriesWhere string, args []any) {
	args = chargs.RollupRangeArgs(f.TeamID, f.StartMs, f.EndMs)
	if len(f.Services) == 1 {
		seriesWhere = "AND s.service = @serviceName"
		args = append(args, clickhouse.Named("serviceName", f.Services[0]))
	} else if len(f.Services) > 1 {
		seriesWhere = "AND s.service IN @services"
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

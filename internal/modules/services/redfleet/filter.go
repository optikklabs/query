package redfleet

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/shared/chargs"
)

type REDFilters struct {
	TenantID int64
	StartMs  int64
	EndMs    int64
	Services []string
}

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

func (f REDFilters) SingleService() string {
	if len(f.Services) == 1 {
		return f.Services[0]
	}
	return ""
}

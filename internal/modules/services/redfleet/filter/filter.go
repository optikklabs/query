package filter

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type Filters struct {
	TenantID int64
	StartMs  int64
	EndMs    int64
	Services []string
}

func BuildClauses(f Filters) (where string, args []any) {
	where, args = BuildServiceClauses(f)
	return " AND " + spanstats.InboundPred + where, args
}

// Outbound queries scope by service without the inbound filter.
func BuildServiceClauses(f Filters) (where string, args []any) {
	args = chargs.RangeArgs(f.TenantID, f.StartMs, f.EndMs)
	if len(f.Services) == 1 {
		where = " AND service = @serviceName"
		args = append(args, clickhouse.Named("serviceName", f.Services[0]))
	} else if len(f.Services) > 1 {
		where = " AND service IN @services"
		args = append(args, clickhouse.Named("services", f.Services))
	}
	return where, args
}

func (f Filters) SingleService() string {
	if len(f.Services) == 1 {
		return f.Services[0]
	}
	return ""
}

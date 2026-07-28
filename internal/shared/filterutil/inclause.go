package filterutil

import (
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type InClause struct {
	Column string
	Bind   string
	Values []string
	Negate bool
}

func AppendIn(dst *string, args []any, clauses ...InClause) []any {
	for _, c := range clauses {
		if len(c.Values) == 0 {
			continue
		}
		op := " IN @"
		if c.Negate {
			op = " NOT IN @"
		}
		*dst += " AND " + c.Column + op + c.Bind
		args = append(args, clickhouse.Named(c.Bind, c.Values))
	}
	return args
}

func UpperAll(vs []string) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = strings.ToUpper(v)
	}
	return out
}

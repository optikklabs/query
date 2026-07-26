package filterutil

import (
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// InClause is one "<column> IN @<bind>" set-membership predicate.
//
// Every signal's filter builder is mostly a run of these, previously written
// out by hand as a four-line guard-append-bind block per field. Expressing
// them as data removes the two mistakes that shape invites: forgetting the
// empty-values guard, and binding a name the clause does not reference.
type InClause struct {
	Column string   // SQL column or expression, e.g. "service"
	Bind   string   // bind name, without the leading @
	Values []string // predicate is skipped entirely when empty
	Negate bool     // emit NOT IN
}

// AppendIn appends each non-empty clause to *dst and returns args extended
// with the matching binds. Clauses with no values emit nothing at all, so a
// caller can list every filter field unconditionally.
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

// UpperAll returns vs uppercased. Severity is stored with the producer's
// casing but compared against upper(severity_text).
func UpperAll(vs []string) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = strings.ToUpper(v)
	}
	return out
}

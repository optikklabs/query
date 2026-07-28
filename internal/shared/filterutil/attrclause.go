package filterutil

import (
	"strconv"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// AttrSQL adapts BuildAttrClause to a signal's attribute schema. Traces
// store attributes in one nullable string map; logs use typed
// attributes_string/number/bool maps, so eq/neq and existence differ.
// Every expression func receives the key bind name (e.g. "akey_0").
type AttrSQL struct {
	StringExpr    func(k string) string // string-value access expression
	NumberExpr    func(k string) string // Float64-or-NULL expression for comparisons
	ExistsExpr    func(k string) string // attribute-present predicate
	NotExistsExpr func(k string) string // attribute-absent predicate
	// EqExpr builds eq (negate=false) and neq (negate=true); per-signal
	// because logs also match typed number/bool values, traces do not.
	EqExpr func(af AttrFilter, k, v string, keyArg any, negate bool) (string, []any)
}

// BuildAttrClause renders one attribute filter as an " AND ..." SQL
// fragment plus its named args, binding @akey_<i> / @aval_<i>.
func BuildAttrClause(s AttrSQL, af AttrFilter, i int) (string, []any) {
	idx := strconv.Itoa(i)
	k := "akey_" + idx
	v := "aval_" + idx
	keyArg := clickhouse.Named(k, af.Key)
	strArgs := []any{keyArg, clickhouse.Named(v, af.Value)}

	switch af.Op {
	case "", "eq":
		return s.EqExpr(af, k, v, keyArg, false)
	case "neq":
		return s.EqExpr(af, k, v, keyArg, true)
	case "contains":
		return ` AND positionCaseInsensitive(` + s.StringExpr(k) + `, @` + v + `) > 0`, strArgs
	case "regex":
		return ` AND match(` + s.StringExpr(k) + `, @` + v + `)`, strArgs
	case "gt", "gte", "lt", "lte":
		n, _ := strconv.ParseFloat(af.Value, 64)
		return ` AND ` + s.NumberExpr(k) + ` ` + CmpSQL(af.Op) + ` @` + v,
			[]any{keyArg, clickhouse.Named(v, n)}
	case "exists":
		return ` AND ` + s.ExistsExpr(k), []any{keyArg}
	case "not_exists":
		return ` AND ` + s.NotExistsExpr(k), []any{keyArg}
	}
	return "", nil
}

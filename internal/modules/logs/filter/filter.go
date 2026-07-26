package filter

import (
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

const maxTimeRangeMs = filterutil.MaxTimeRangeMs

type AttrFilter = filterutil.AttrFilter

type Filters struct {
	TenantID int64 `json:"-"`
	StartMs  int64 `json:"-"`
	EndMs    int64 `json:"-"`

	Services     []string `json:"services,omitempty"`
	Hosts        []string `json:"hosts,omitempty"`
	Pods         []string `json:"pods,omitempty"`
	Containers   []string `json:"containers,omitempty"`
	Environments []string `json:"environments,omitempty"`
	Severities   []string `json:"severities,omitempty"`

	TraceID    string `json:"traceId,omitempty"`
	SpanID     string `json:"spanId,omitempty"`
	Search     string `json:"search,omitempty"`
	SearchMode string `json:"searchMode,omitempty"`

	ExcludeServices   []string `json:"excludeServices,omitempty"`
	ExcludeHosts      []string `json:"excludeHosts,omitempty"`
	ExcludeSeverities []string `json:"excludeSeverities,omitempty"`

	Attributes []AttrFilter `json:"attributes,omitempty"`
}

func (f *Filters) Validate() error {
	if err := filterutil.ValidateTimeRange(&f.StartMs, &f.EndMs); err != nil {
		return err
	}
	return filterutil.ValidateAttrs(f.Attributes)
}

var ValidateAttrs = filterutil.ValidateAttrs

func BuildClauses(f Filters) (prewhere, where string, args []any) {
	startBucket := uint32((f.StartMs / 1000) / 300 * 300)
	endBucket := uint32((f.EndMs / 1000) / 300 * 300)

	args = []any{
		clickhouse.Named("tenantID", uint32(f.TenantID)),
		clickhouse.Named("start", time.UnixMilli(f.StartMs)),
		clickhouse.Named("end", time.UnixMilli(f.EndMs)),
		clickhouse.Named("startBucket", startBucket),
		clickhouse.Named("endBucket", endBucket),
	}

	prewhere = `PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND ts_bucket BETWEEN @startBucket AND @endBucket`
	where = `WHERE 1=1`

	// Resource dimensions are all PREWHERE: they are low-cardinality columns
	// in the sort key, so pruning on them before the body scan is the point.
	args = filterutil.AppendIn(&prewhere, args,
		filterutil.InClause{Column: "service", Bind: "services", Values: f.Services},
		filterutil.InClause{Column: "service", Bind: "excServices", Values: f.ExcludeServices, Negate: true},
		filterutil.InClause{Column: "host", Bind: "hosts", Values: f.Hosts},
		filterutil.InClause{Column: "host", Bind: "excHosts", Values: f.ExcludeHosts, Negate: true},
		filterutil.InClause{Column: "pod", Bind: "pods", Values: f.Pods},
		filterutil.InClause{Column: "container", Bind: "containers", Values: f.Containers},
		filterutil.InClause{Column: "environment", Bind: "environments", Values: f.Environments},
	)

	args = filterutil.AppendIn(&where, args,
		filterutil.InClause{Column: "upper(severity_text)", Bind: "severities",
			Values: filterutil.UpperAll(f.Severities)},
		filterutil.InClause{Column: "upper(severity_text)", Bind: "excSeverities",
			Values: filterutil.UpperAll(f.ExcludeSeverities), Negate: true},
	)

	if f.TraceID != "" {
		where += ` AND trace_id = @traceID`
		args = append(args, clickhouse.Named("traceID", f.TraceID))
	}
	if f.SpanID != "" {
		where += ` AND span_id = @spanID`
		args = append(args, clickhouse.Named("spanID", f.SpanID))
	}
	if f.Search != "" {
		// Case-insensitive substring: the only search semantic. The legacy
		// searchMode field is still accepted on the wire but ignored.
		// LIKE (not position*) so the idx_body_ngram skip index can prune.
		where += ` AND lowerUTF8(body) LIKE @search`
		args = append(args, clickhouse.Named("search", likeSubstringPattern(f.Search)))
	}
	for i, af := range f.Attributes {
		clause, clauseArgs := buildAttrClause(af, i)
		where += clause
		args = append(args, clauseArgs...)
	}
	return prewhere, where, args
}

// likeSubstringPattern turns a raw search term into a %term% LIKE pattern,
// lowercased to match the lowerUTF8(body) column expression and with LIKE
// metacharacters escaped so user input always means a literal substring.
func likeSubstringPattern(term string) string {
	esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.ToLower(term))
	return "%" + esc + "%"
}

// buildAttrClause emits the WHERE predicate for one attribute filter.
// Values live in three typed maps; eq/neq check the string map and, when the
// value parses as a number or bool, the matching typed map as well, so a
// filter works regardless of how the producer typed the attribute.
func buildAttrClause(af AttrFilter, i int) (string, []any) {
	idx := strconv.Itoa(i)
	k := "akey_" + idx
	v := "aval_" + idx
	keyArg := clickhouse.Named(k, af.Key)

	switch af.Op {
	case "", "eq":
		return buildAttrEqClause(af, k, v, keyArg, false)
	case "neq":
		return buildAttrEqClause(af, k, v, keyArg, true)
	case "contains":
		return ` AND positionCaseInsensitive(attributes_string[@` + k + `], @` + v + `) > 0`,
			[]any{keyArg, clickhouse.Named(v, af.Value)}
	case "regex":
		return ` AND match(attributes_string[@` + k + `], @` + v + `)`,
			[]any{keyArg, clickhouse.Named(v, af.Value)}
	case "gt", "gte", "lt", "lte":
		// Validate guarantees the value parses; NULL (missing key or
		// non-numeric string value) compares false and drops the row.
		n, _ := strconv.ParseFloat(af.Value, 64)
		expr := `coalesce(toFloat64OrNull(attributes_string[@` + k + `]),` +
			` if(mapContains(attributes_number, @` + k + `), attributes_number[@` + k + `], NULL))`
		return ` AND ` + expr + ` ` + filterutil.CmpSQL(af.Op) + ` @` + v,
			[]any{keyArg, clickhouse.Named(v, n)}
	case "exists":
		return ` AND ` + attrExistsExpr(k), []any{keyArg}
	case "not_exists":
		return ` AND NOT ` + attrExistsExpr(k), []any{keyArg}
	}
	return "", nil // unreachable: ops are whitelisted in Validate
}

// buildAttrEqClause handles eq/neq. neq requires the key to exist so that
// "not equals" never matches rows missing the attribute entirely.
func buildAttrEqClause(af AttrFilter, k, v string, keyArg any, negate bool) (string, []any) {
	op := "="
	if negate {
		op = "!="
	}
	strCmp := `attributes_string[@` + k + `] ` + op + ` @` + v
	if negate {
		strCmp = `(mapContains(attributes_string, @` + k + `) AND ` + strCmp + `)`
	}
	args := []any{keyArg, clickhouse.Named(v, af.Value)}

	if n, err := strconv.ParseFloat(af.Value, 64); err == nil {
		vn := v + "_n"
		clause := `(` + strCmp + ` OR (mapContains(attributes_number, @` + k + `)` +
			` AND attributes_number[@` + k + `] ` + op + ` @` + vn + `))`
		return ` AND ` + clause, append(args, clickhouse.Named(vn, n))
	}
	if b, err := strconv.ParseBool(af.Value); err == nil {
		vb := v + "_b"
		clause := `(` + strCmp + ` OR (mapContains(attributes_bool, @` + k + `)` +
			` AND attributes_bool[@` + k + `] ` + op + ` @` + vb + `))`
		return ` AND ` + clause, append(args, clickhouse.Named(vb, b))
	}
	return ` AND ` + strCmp, args
}

func attrExistsExpr(k string) string {
	return `(mapContains(attributes_string, @` + k + `)` +
		` OR mapContains(attributes_number, @` + k + `)` +
		` OR mapContains(attributes_bool, @` + k + `))`
}

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

// BuildFingerprintCTE turns a resource filter into a fingerprint-pruning CTE.
// It resolves matching fingerprints from the small logs_resource dimension table
// so the logs table scan can be pruned by its primary key. Both return values are
// empty when there is no resource filter.
func BuildFingerprintCTE(resourceWhere string) (cte, prewhereFP string) {
	if resourceWhere == "" {
		return "", ""
	}
	cte = `
		WITH active_fps AS (
		    SELECT DISTINCT fingerprint
		    FROM optikk.logs_resource
		    PREWHERE tenant_id = @tenantID` + resourceWhere + `
		)`
	prewhereFP = " AND fingerprint IN active_fps"
	return cte, prewhereFP
}

func (f Filters) HasResourceFilters() bool {
	return len(f.Services) > 0 ||
		len(f.ExcludeServices) > 0 ||
		len(f.Hosts) > 0 ||
		len(f.ExcludeHosts) > 0 ||
		len(f.Pods) > 0 ||
		len(f.Containers) > 0 ||
		len(f.Environments) > 0
}

func BuildClauses(f Filters) (resourceWhere, where string, args []any) {
	startBucket := uint32((f.StartMs / 1000) / 300 * 300)
	endBucket := uint32((f.EndMs / 1000) / 300 * 300)

	args = []any{
		clickhouse.Named("tenantID", uint32(f.TenantID)),
		clickhouse.Named("start", time.UnixMilli(f.StartMs)),
		clickhouse.Named("end", time.UnixMilli(f.EndMs)),
		clickhouse.Named("startBucket", startBucket),
		clickhouse.Named("endBucket", endBucket),
	}

	if f.HasResourceFilters() {
		resourceWhere += ` AND ts_bucket BETWEEN @startBucket AND @endBucket`
	}
	where += ` AND ts_bucket BETWEEN @startBucket AND @endBucket AND timestamp BETWEEN @start AND @end`

	if len(f.Services) > 0 {
		resourceWhere += ` AND service IN @services`
		args = append(args, clickhouse.Named("services", f.Services))
	}
	if len(f.ExcludeServices) > 0 {
		resourceWhere += ` AND service NOT IN @excServices`
		args = append(args, clickhouse.Named("excServices", f.ExcludeServices))
	}
	if len(f.Hosts) > 0 {
		resourceWhere += ` AND host IN @hosts`
		args = append(args, clickhouse.Named("hosts", f.Hosts))
	}
	if len(f.ExcludeHosts) > 0 {
		resourceWhere += ` AND host NOT IN @excHosts`
		args = append(args, clickhouse.Named("excHosts", f.ExcludeHosts))
	}
	if len(f.Pods) > 0 {
		resourceWhere += ` AND pod IN @pods`
		args = append(args, clickhouse.Named("pods", f.Pods))
	}
	if len(f.Containers) > 0 {
		resourceWhere += ` AND container IN @containers`
		args = append(args, clickhouse.Named("containers", f.Containers))
	}
	if len(f.Environments) > 0 {
		resourceWhere += ` AND environment IN @environments`
		args = append(args, clickhouse.Named("environments", f.Environments))
	}

	if len(f.Severities) > 0 {
		where += ` AND upper(severity_text) IN @severities`
		args = append(args, clickhouse.Named("severities", upperAll(f.Severities)))
	}
	if len(f.ExcludeSeverities) > 0 {
		where += ` AND upper(severity_text) NOT IN @excSeverities`
		args = append(args, clickhouse.Named("excSeverities", upperAll(f.ExcludeSeverities)))
	}
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
		where += ` AND positionCaseInsensitive(body, @search) > 0`
		args = append(args, clickhouse.Named("search", f.Search))
	}
	for i, af := range f.Attributes {
		clause, clauseArgs := buildAttrClause(af, i)
		where += clause
		args = append(args, clauseArgs...)
	}
	return resourceWhere, where, args
}

func upperAll(vs []string) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = strings.ToUpper(v)
	}
	return out
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

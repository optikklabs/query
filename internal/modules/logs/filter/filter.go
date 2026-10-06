package filter

import (
	"strconv"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

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

	TraceID string `json:"traceId,omitempty"`
	SpanID  string `json:"spanId,omitempty"`
	Search  string `json:"search,omitempty"`

	ExcludeServices   []string `json:"excludeServices,omitempty"`
	ExcludeHosts      []string `json:"excludeHosts,omitempty"`
	ExcludeSeverities []string `json:"excludeSeverities,omitempty"`

	Attributes []AttrFilter `json:"attributes,omitempty"`
}

func (f *Filters) Validate() error {
	if err := filterutil.ValidateTimeRange(f.StartMs, f.EndMs); err != nil {
		return err
	}
	return filterutil.ValidateAttrs(f.Attributes)
}

// RangeRequest is the JSON body the log explorer endpoints share: a time range
// plus the filter set.
type RangeRequest struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`

	Filters
}

// BindTenant scopes the filters to the tenant and the requested range.
func (r *RangeRequest) BindTenant(tenantID int64) error {
	r.TenantID = tenantID
	r.StartMs = r.StartTime
	r.EndMs = r.EndTime
	return r.Validate()
}

// rangeArgs binds the tenant, the time window and its ts_bucket bounds.
func rangeArgs(f Filters) []any {
	return []any{
		clickhouse.Named("tenantID", uint32(f.TenantID)),
		chargs.Millis("start", f.StartMs),
		chargs.Millis("end", f.EndMs),
		clickhouse.Named("startBucket", timebucket.LogBucket(f.StartMs)),
		clickhouse.Named("endBucket", timebucket.LogBucket(f.EndMs)),
	}
}

// resourceIns and severityIns are the IN filters shared by the raw and
// rollup scans.
func resourceIns(f Filters) []filterutil.InClause {
	return []filterutil.InClause{
		{Column: "service", Bind: "services", Values: f.Services},
		{Column: "service", Bind: "excServices", Values: f.ExcludeServices, Negate: true},
		{Column: "host", Bind: "hosts", Values: f.Hosts},
		{Column: "host", Bind: "excHosts", Values: f.ExcludeHosts, Negate: true},
		{Column: "pod", Bind: "pods", Values: f.Pods},
		{Column: "container", Bind: "containers", Values: f.Containers},
		{Column: "environment", Bind: "environments", Values: f.Environments},
	}
}

func severityIns(f Filters) []filterutil.InClause {
	return []filterutil.InClause{
		{Column: "upper(severity_text)", Bind: "severities", Values: filterutil.UpperAll(f.Severities)},
		{Column: "upper(severity_text)", Bind: "excSeverities", Values: filterutil.UpperAll(f.ExcludeSeverities), Negate: true},
	}
}

func BuildClauses(f Filters) (prewhere, where string, args []any) {
	args = rangeArgs(f)
	prewhere = `PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end AND ts_bucket BETWEEN @startBucket AND @endBucket`
	where = `WHERE 1=1`

	args = filterutil.AppendIn(&prewhere, args, resourceIns(f)...)
	args = filterutil.AppendIn(&where, args, severityIns(f)...)

	if f.TraceID != "" {
		where += ` AND trace_id = @traceID`
		args = append(args, clickhouse.Named("traceID", f.TraceID))
	}
	if f.SpanID != "" {
		where += ` AND span_id = @spanID`
		args = append(args, clickhouse.Named("spanID", f.SpanID))
	}
	if f.Search != "" {
		where += ` AND lowerUTF8(body) LIKE @search`
		args = append(args, clickhouse.Named("search", filterutil.LikeSubstringPattern(f.Search)))
	}
	for i, af := range f.Attributes {
		clause, clauseArgs := buildAttrClause(af, i)
		where += clause
		args = append(args, clauseArgs...)
	}
	return prewhere, where, args
}

const statsGrainMs int64 = 60 * 1000

type StatsClauses struct {
	RawPrewhere    string
	RollupPrewhere string
	Where          string
	Args           []any
}

// SupportsStats reports whether every requested predicate is represented in
// logs_stats_1m. Body, ID, and arbitrary-attribute predicates remain raw-only.
func SupportsStats(f Filters) bool {
	return f.TraceID == "" && f.SpanID == "" && f.Search == "" && len(f.Attributes) == 0
}

// BuildStatsClauses splits an exact aggregate read into complete minutes from
// logs_stats_1m and at most two partial-minute fragments from raw logs. This is
// a single deterministic query plan, not an age- or retention-based fallback.
func BuildStatsClauses(f Filters) (StatsClauses, bool) {
	if !SupportsStats(f) {
		return StatsClauses{}, false
	}
	rollupStartMs := ((f.StartMs + statsGrainMs - 1) / statsGrainMs) * statsGrainMs
	rollupEndMs := (f.EndMs / statsGrainMs) * statsGrainMs
	if rollupStartMs >= rollupEndMs {
		return StatsClauses{}, false
	}

	args := append(rangeArgs(f),
		chargs.Millis("rollupStart", rollupStartMs),
		chargs.Millis("rollupEnd", rollupEndMs),
	)
	dimensions := ""
	args = filterutil.AppendIn(&dimensions, args, resourceIns(f)...)
	where := "WHERE 1=1"
	args = filterutil.AppendIn(&where, args, severityIns(f)...)

	return StatsClauses{
		RawPrewhere: `PREWHERE tenant_id = @tenantID
			AND timestamp >= @start AND timestamp < @end
			AND ts_bucket BETWEEN @startBucket AND @endBucket
			AND (timestamp < @rollupStart OR timestamp >= @rollupEnd)` + dimensions,
		RollupPrewhere: `PREWHERE tenant_id = @tenantID
			AND timestamp >= @rollupStart AND timestamp < @rollupEnd` + dimensions,
		Where: where,
		Args:  args,
	}, true
}

// attrSQL: logs split attributes into typed string/number/bool maps;
// unlike traces, eq/neq also match typed number/bool values.
var attrSQL = filterutil.AttrSQL{
	StringExpr: func(k string) string {
		return `if(mapContains(attributes_string, @` + k + `), attributes_string[@` + k + `], NULL)`
	},
	NumberExpr: func(k string) string {
		return `coalesce(toFloat64OrNull(attributes_string[@` + k + `]),` +
			` if(mapContains(attributes_number, @` + k + `), attributes_number[@` + k + `], NULL))`
	},
	ExistsExpr:    attrExistsExpr,
	NotExistsExpr: func(k string) string { return `NOT ` + attrExistsExpr(k) },
	EqExpr:        buildAttrEqClause,
}

func buildAttrClause(af AttrFilter, i int) (string, []any) {
	return filterutil.BuildAttrClause(attrSQL, af, i)
}

func buildAttrEqClause(af AttrFilter, k, v string, keyArg any, negate bool) (string, []any) {
	op := "="
	if negate {
		op = "!="
	}
	strCmp := `(mapContains(attributes_string, @` + k + `) AND attributes_string[@` + k + `] ` + op + ` @` + v + `)`
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

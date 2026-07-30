package filter

import (
	"errors"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
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
	if f.EndMs-f.StartMs > filterutil.RawRetentionMs {
		return errors.New("filters: log data is retained for 15 days")
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

	prewhere = `PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end AND ts_bucket BETWEEN @startBucket AND @endBucket`
	where = `WHERE 1=1`

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

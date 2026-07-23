// Package filter defines the traces filter shape, validation, and SQL clause
// emitter.
package filter

import (
	"strconv"
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

	Services      []string `json:"services,omitempty"`
	Operations    []string `json:"operations,omitempty"`
	SpanKinds     []string `json:"spanKinds,omitempty"`
	HTTPMethods   []string `json:"httpMethods,omitempty"`
	HTTPStatuses  []string `json:"httpStatuses,omitempty"`
	Statuses      []string `json:"statuses,omitempty"`
	Environments  []string `json:"environments,omitempty"`
	PeerServices  []string `json:"peerServices,omitempty"`
	TraceID       string   `json:"traceId,omitempty"`
	MinDurationNs int64    `json:"minDurationNs,omitempty"`
	MaxDurationNs int64    `json:"maxDurationNs,omitempty"`
	HasError      *bool    `json:"hasError,omitempty"`

	ExcludeServices []string `json:"excludeServices,omitempty"`
	ExcludeStatuses []string `json:"excludeStatuses,omitempty"`

	Search     string `json:"search,omitempty"`
	SearchMode string `json:"searchMode,omitempty"`

	Attributes []AttrFilter `json:"attributes,omitempty"`
}

func (f *Filters) Validate() error {
	if err := filterutil.ValidateTimeRange(&f.StartMs, &f.EndMs); err != nil {
		return err
	}
	return filterutil.ValidateAttrs(f.Attributes)
}

var ValidateAttrs = filterutil.ValidateAttrs

// Clauses splits the WHERE predicates by where they must be evaluated.
//
// Span predicates match ANY span of a trace (service:X means "traces that
// touch X"), so the repository runs them in a trace_id subquery over all
// spans. Root predicates are trace-level (duration, exclusions) and stay on
// the root-span scan. Resource contains predicates applied directly to the
// inner any-span spans scan PREWHERE.
type Clauses struct {
	Resource string // predicates appended to inner spans scan PREWHERE (service IN)
	Span     string // predicates matchable against any span
	Root     string // trace-level predicates, evaluated on the root span
	Args     []any
}

// HasSpanMatch reports whether the any-span subquery phase is needed.
func (c Clauses) HasSpanMatch() bool {
	return c.Span != ""
}

func BuildClauses(f Filters) Clauses {
	c := Clauses{Args: []any{
		clickhouse.Named("tenantID", uint32(f.TenantID)),
		clickhouse.Named("start", time.UnixMilli(f.StartMs)),
		clickhouse.Named("end", time.UnixMilli(f.EndMs)),
	}}

	if len(f.Services) > 0 {
		c.Root += ` AND service IN @services`
		c.Resource += ` AND service IN @services`
		c.Args = append(c.Args, clickhouse.Named("services", f.Services))
	}
	if len(f.Environments) > 0 {
		c.Span += ` AND environment IN @environments`
		c.Args = append(c.Args, clickhouse.Named("environments", f.Environments))
	}
	if len(f.ExcludeServices) > 0 {
		c.Root += ` AND service NOT IN @excServices`
		c.Args = append(c.Args, clickhouse.Named("excServices", f.ExcludeServices))
	}

	if len(f.Operations) > 0 {
		c.Span += ` AND name IN @operations`
		c.Args = append(c.Args, clickhouse.Named("operations", f.Operations))
	}
	if len(f.SpanKinds) > 0 {
		c.Span += ` AND kind_string IN @spanKinds`
		c.Args = append(c.Args, clickhouse.Named("spanKinds", f.SpanKinds))
	}
	if len(f.HTTPMethods) > 0 {
		c.Span += ` AND http_method IN @httpMethods`
		c.Args = append(c.Args, clickhouse.Named("httpMethods", f.HTTPMethods))
	}
	if len(f.HTTPStatuses) > 0 {
		c.Span += ` AND response_status_code IN @httpStatuses`
		c.Args = append(c.Args, clickhouse.Named("httpStatuses", f.HTTPStatuses))
	}
	if len(f.Statuses) > 0 {
		c.Span += ` AND status_code_string IN @statuses`
		c.Args = append(c.Args, clickhouse.Named("statuses", f.Statuses))
	}
	if len(f.PeerServices) > 0 {
		c.Span += ` AND peer_service IN @peerServices`
		c.Args = append(c.Args, clickhouse.Named("peerServices", f.PeerServices))
	}
	if f.Search != "" {
		// Case-insensitive substring on span name; matches any span of the
		// trace. Legacy searchMode is accepted on the wire but ignored.
		c.Span += ` AND positionCaseInsensitive(name, @search) > 0`
		c.Args = append(c.Args, clickhouse.Named("search", f.Search))
	}
	for i, af := range f.Attributes {
		clause, clauseArgs := buildAttrClause(af, i)
		c.Span += clause
		c.Args = append(c.Args, clauseArgs...)
	}

	if len(f.ExcludeStatuses) > 0 {
		c.Root += ` AND status_code_string NOT IN @excStatuses`
		c.Args = append(c.Args, clickhouse.Named("excStatuses", f.ExcludeStatuses))
	}
	if f.TraceID != "" {
		c.Root += ` AND trace_id = @traceID`
		c.Args = append(c.Args, clickhouse.Named("traceID", f.TraceID))
	}
	// Duration means trace duration, i.e. the root span's duration.
	if f.MinDurationNs > 0 {
		c.Root += ` AND duration_nano >= @minDur`
		c.Args = append(c.Args, clickhouse.Named("minDur", uint64(f.MinDurationNs)))
	}
	if f.MaxDurationNs > 0 {
		c.Root += ` AND duration_nano <= @maxDur`
		c.Args = append(c.Args, clickhouse.Named("maxDur", uint64(f.MaxDurationNs)))
	}
	if f.HasError != nil {
		if *f.HasError {
			// "has an error anywhere in the trace" — any-span predicate.
			c.Span += ` AND has_error = 1`
		} else {
			// "root completed cleanly" — kept root-level; "no error on any
			// span" is not expressible as an any-span predicate.
			c.Root += ` AND has_error = 0`
		}
	}
	return c
}

// buildAttrClause emits the predicate for one attribute filter against the
// spans JSON attributes column. Missing paths read as NULL, so equality and
// comparisons drop rows without the key; neq/exists guard explicitly.
func buildAttrClause(af AttrFilter, i int) (string, []any) {
	idx := strconv.Itoa(i)
	k := "akey_" + idx
	v := "aval_" + idx
	keyArg := clickhouse.Named(k, af.Key)
	strArgs := []any{keyArg, clickhouse.Named(v, af.Value)}

	switch af.Op {
	case "", "eq":
		return ` AND attributes[@` + k + `] = @` + v, strArgs
	case "neq":
		return ` AND (NOT (attributes[@` + k + `] IS NULL) AND attributes[@` + k + `] != @` + v + `)`, strArgs
	case "contains":
		return ` AND positionCaseInsensitive(attributes[@` + k + `], @` + v + `) > 0`, strArgs
	case "regex":
		return ` AND match(attributes[@` + k + `], @` + v + `)`, strArgs
	case "gt", "gte", "lt", "lte":
		n, _ := strconv.ParseFloat(af.Value, 64)
		// Note: since all Map values are Strings, numeric comparisons require casting
		return ` AND toFloat64OrNull(attributes[@` + k + `]) ` + filterutil.CmpSQL(af.Op) + ` @` + v,
			[]any{keyArg, clickhouse.Named(v, n)}
	case "exists":
		return ` AND NOT (attributes[@` + k + `] IS NULL)`, []any{keyArg}
	case "not_exists":
		return ` AND attributes[@` + k + `] IS NULL`, []any{keyArg}
	}
	return "", nil // unreachable: ops are whitelisted in Validate
}

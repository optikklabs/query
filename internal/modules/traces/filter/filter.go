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

type Clauses struct {
	Resource string
	Span     string
	Root     string
	Args     []any
}

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

	c.Args = filterutil.AppendIn(&c.Span, c.Args,
		filterutil.InClause{Column: "environment", Bind: "environments", Values: f.Environments},
		filterutil.InClause{Column: "name", Bind: "operations", Values: f.Operations},
		filterutil.InClause{Column: "kind_string", Bind: "spanKinds", Values: f.SpanKinds},
		filterutil.InClause{Column: "http_method", Bind: "httpMethods", Values: f.HTTPMethods},
		filterutil.InClause{Column: "response_status_code", Bind: "httpStatuses", Values: f.HTTPStatuses},
		filterutil.InClause{Column: "status_code_string", Bind: "statuses", Values: f.Statuses},
		filterutil.InClause{Column: "peer_service", Bind: "peerServices", Values: f.PeerServices},
	)

	c.Args = filterutil.AppendIn(&c.Root, c.Args,
		filterutil.InClause{Column: "service", Bind: "excServices", Values: f.ExcludeServices, Negate: true},
	)

	if f.Search != "" {

		c.Span += ` AND positionCaseInsensitive(name, @search) > 0`
		c.Args = append(c.Args, clickhouse.Named("search", f.Search))
	}
	for i, af := range f.Attributes {
		clause, clauseArgs := buildAttrClause(af, i)
		c.Span += clause
		c.Args = append(c.Args, clauseArgs...)
	}

	c.Args = filterutil.AppendIn(&c.Root, c.Args,
		filterutil.InClause{Column: "status_code_string", Bind: "excStatuses",
			Values: f.ExcludeStatuses, Negate: true},
	)
	if f.TraceID != "" {
		c.Root += ` AND trace_id = @traceID`
		c.Args = append(c.Args, clickhouse.Named("traceID", f.TraceID))
	}

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

			c.Span += ` AND has_error = 1`
		} else {

			c.Root += ` AND has_error = 0`
		}
	}
	return c
}

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

		return ` AND toFloat64OrNull(attributes[@` + k + `]) ` + filterutil.CmpSQL(af.Op) + ` @` + v,
			[]any{keyArg, clickhouse.Named(v, n)}
	case "exists":
		return ` AND NOT (attributes[@` + k + `] IS NULL)`, []any{keyArg}
	case "not_exists":
		return ` AND attributes[@` + k + `] IS NULL`, []any{keyArg}
	}
	return "", nil
}

package spanfilter

import (
	"errors"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

type AttrFilter = filterutil.AttrFilter

type Filters struct {
	TenantID int64 `json:"-"`
	StartMs  int64 `json:"-"`
	EndMs    int64 `json:"-"`

	Services        []string `json:"services,omitempty"`
	ServiceVersions []string `json:"serviceVersions,omitempty"`
	Operations      []string `json:"operations,omitempty"`
	SpanKinds       []string `json:"spanKinds,omitempty"`
	HTTPMethods     []string `json:"httpMethods,omitempty"`
	HTTPStatuses    []string `json:"httpStatuses,omitempty"`
	Statuses        []string `json:"statuses,omitempty"`
	Environments    []string `json:"environments,omitempty"`
	PeerServices    []string `json:"peerServices,omitempty"`
	ExceptionType   []string `json:"exceptionTypes,omitempty"`
	TraceID         string   `json:"traceId,omitempty"`
	MinDurationNs   int64    `json:"minDurationNs,omitempty"`
	MaxDurationNs   int64    `json:"maxDurationNs,omitempty"`
	HasError        *bool    `json:"hasError,omitempty"`

	ExcludeServices []string `json:"excludeServices,omitempty"`
	ExcludeStatuses []string `json:"excludeStatuses,omitempty"`

	// Search matches the span name; Message matches the span status message.
	Search     string `json:"search,omitempty"`
	SearchMode string `json:"searchMode,omitempty"`
	Message    string `json:"message,omitempty"`

	Attributes []AttrFilter `json:"attributes,omitempty"`
}

func (f *Filters) Validate() error {
	if err := filterutil.ValidateTimeRange(&f.StartMs, &f.EndMs); err != nil {
		return err
	}
	if f.EndMs-f.StartMs > filterutil.RawRetentionMs {
		return errors.New("filters: span data is retained for 15 days")
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
	c.Args = filterutil.AppendIn(&c.Root, c.Args,
		filterutil.InClause{Column: "service_version", Bind: "serviceVersions", Values: f.ServiceVersions},
	)

	c.Args = filterutil.AppendIn(&c.Span, c.Args,
		filterutil.InClause{Column: "environment", Bind: "environments", Values: f.Environments},
		filterutil.InClause{Column: "name", Bind: "operations", Values: f.Operations},
		filterutil.InClause{Column: "kind_string", Bind: "spanKinds", Values: f.SpanKinds},
		filterutil.InClause{Column: "http_method", Bind: "httpMethods", Values: f.HTTPMethods},
		filterutil.InClause{Column: "response_status_code", Bind: "httpStatuses", Values: f.HTTPStatuses},
		filterutil.InClause{Column: "status_code_string", Bind: "statuses", Values: f.Statuses},
		filterutil.InClause{Column: "peer_service", Bind: "peerServices", Values: f.PeerServices},
		filterutil.InClause{Column: "exception_type", Bind: "exceptionTypes", Values: f.ExceptionType},
	)

	c.Args = filterutil.AppendIn(&c.Root, c.Args,
		filterutil.InClause{Column: "service", Bind: "excServices", Values: f.ExcludeServices, Negate: true},
	)

	if f.Search != "" {

		c.Span += ` AND positionCaseInsensitive(name, @search) > 0`
		c.Args = append(c.Args, clickhouse.Named("search", f.Search))
	}
	if f.Message != "" {
		c.Span += ` AND positionCaseInsensitive(status_message, @message) > 0`
		c.Args = append(c.Args, clickhouse.Named("message", f.Message))
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
		op := " NOT IN "
		if *f.HasError {
			op = " IN "
		}
		c.Root += ` AND trace_id` + op + `(SELECT trace_id FROM optikk.spans
			PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
			WHERE is_error = 1)`
	}
	return c
}

// attrSQL: traces keep one string attribute map; unlike logs,
// eq/neq compare the string value only (no typed number/bool matching).
var attrSQL = filterutil.AttrSQL{
	StringExpr:    func(k string) string { return `if(mapContains(attributes, @` + k + `), attributes[@` + k + `], NULL)` },
	NumberExpr:    func(k string) string { return `toFloat64OrNull(attributes[@` + k + `])` },
	ExistsExpr:    func(k string) string { return `mapContains(attributes, @` + k + `)` },
	NotExistsExpr: func(k string) string { return `NOT mapContains(attributes, @` + k + `)` },
	EqExpr:        buildAttrEqClause,
}

func buildAttrEqClause(af AttrFilter, k, v string, keyArg any, negate bool) (string, []any) {
	args := []any{keyArg, clickhouse.Named(v, af.Value)}
	if negate {
		return ` AND (mapContains(attributes, @` + k + `) AND attributes[@` + k + `] != @` + v + `)`, args
	}
	return ` AND (mapContains(attributes, @` + k + `) AND attributes[@` + k + `] = @` + v + `)`, args
}

func buildAttrClause(af AttrFilter, i int) (string, []any) {
	return filterutil.BuildAttrClause(attrSQL, af, i)
}

package spanfilter

import (
	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/optikklabs/query/internal/shared/chargs"
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
	Search  string `json:"search,omitempty"`
	Message string `json:"message,omitempty"`

	Attributes []AttrFilter `json:"attributes,omitempty"`
}

func (f *Filters) Validate() error {
	if err := filterutil.ValidateTimeRange(f.StartMs, f.EndMs); err != nil {
		return err
	}
	return filterutil.ValidateAttrs(f.Attributes)
}

// RangeRequest is the JSON body the span explorer endpoints share: a time range
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

// Clauses split the filters by the rows they test. Root predicates apply to
// the trace's root span. Span predicates select traces with any matching
// span; Resource predicates are the cheap subset of those, put in PREWHERE.
type Clauses struct {
	Resource string
	Span     string
	Root     string
	Args     []any
}

// HasSpanMatch reports whether the filters need an any-span match.
func (c Clauses) HasSpanMatch() bool {
	return c.Span != "" || c.Resource != ""
}

func BuildClauses(f Filters) Clauses {
	c := Clauses{Args: []any{
		clickhouse.Named("tenantID", uint32(f.TenantID)),
		chargs.Millis("start", f.StartMs),
		chargs.Millis("end", f.EndMs),
	}}

	c.Args = filterutil.AppendIn(&c.Resource, c.Args,
		filterutil.InClause{Column: "service", Bind: "services", Values: f.Services},
	)
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
		c.Root += ` AND trace_id` + op + `(SELECT trace_id FROM optikk.error_events
			PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end)`
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

// promotedColumns maps the attribute keys ingest promotes out of the
// attributes map to the column that holds their raw string value. Keys whose
// column holds a derived value (operation, tokens) are left out.
var promotedColumns = map[string]string{
	"http.route":                "http_route",
	"http.method":               "http_method",
	"http.request.method":       "http_method",
	"http.url":                  "http_url",
	"url.full":                  "http_url",
	"http.host":                 "http_host",
	"net.host.name":             "http_host",
	"http.status_code":          "response_status_code",
	"http.response.status_code": "response_status_code",
	"service.name":              "service",
	"service.version":           "service_version",
	"deployment.environment":    "environment",
	"host.name":                 "host",
	"k8s.pod.name":              "pod",
	"peer.service":              "peer_service",
	"db.system":                 "db_system",
	"db.system.name":            "db_system",
	"db.name":                   "db_name",
	"db.namespace":              "db_name",
	"db.statement":              "db_statement",
	"db.query.text":             "db_statement",
	"exception.type":            "exception_type",
	"exception.message":         "exception_message",
	"exception.stacktrace":      "exception_stacktrace",
	"gen_ai.provider.name":      "gen_ai_system",
	"gen_ai.system":             "gen_ai_system",
	"gen_ai.request.model":      "gen_ai_request_model",
	"gen_ai.response.model":     "gen_ai_response_model",
}

// PromotedColumn returns the column holding a promoted attribute key.
func PromotedColumn(key string) (string, bool) {
	col, ok := promotedColumns[key]
	return col, ok
}

// columnAttrSQL matches a promoted key against its column, where an empty
// value means the attribute was absent.
func columnAttrSQL(col string) filterutil.AttrSQL {
	return filterutil.AttrSQL{
		StringExpr:    func(string) string { return `nullIf(` + col + `, '')` },
		NumberExpr:    func(string) string { return `toFloat64OrNull(` + col + `)` },
		ExistsExpr:    func(string) string { return col + ` != ''` },
		NotExistsExpr: func(string) string { return col + ` = ''` },
		EqExpr: func(af AttrFilter, _, v string, _ any, negate bool) (string, []any) {
			args := []any{clickhouse.Named(v, af.Value)}
			if negate {
				return ` AND (` + col + ` != '' AND ` + col + ` != @` + v + `)`, args
			}
			return ` AND ` + col + ` = @` + v, args
		},
	}
}

func buildAttrClause(af AttrFilter, i int) (string, []any) {
	if col, ok := PromotedColumn(af.Key); ok {
		return filterutil.BuildAttrClause(columnAttrSQL(col), af, i)
	}
	return filterutil.BuildAttrClause(attrSQL, af, i)
}

package explorer

import (
	"github.com/optikklabs/query/internal/shared/filterutil"
)

type Trace struct {
	TraceID        string   `json:"traceId"`
	StartMs        uint64   `json:"startMs"`
	EndMs          uint64   `json:"endMs"`
	DurationMs     float64  `json:"durationMs"`
	RootService    string   `json:"rootService"`
	RootOperation  string   `json:"rootOperation"`
	RootStatus     string   `json:"rootStatus,omitempty"`
	RootHTTPMethod string   `json:"rootHttpMethod,omitempty"`
	RootHTTPStatus string   `json:"rootHttpStatus,omitempty"`
	RootEndpoint   string   `json:"rootEndpoint,omitempty"`
	Environment    string   `json:"environment,omitempty"`
	SpanCount      uint32   `json:"spanCount"`
	HasError       bool     `json:"hasError"`
	ErrorCount     uint32   `json:"errorCount"`
	ServiceSet     []string `json:"serviceSet"`
}

type TraceCursor struct {
	StartNs uint64 `json:"s"`
	TraceID string `json:"t"`
	SpanID  string `json:"p"`
}

type FacetBucket struct {
	Value string `json:"value"`
	Count uint64 `json:"count"`
}

type Facets struct {
	Service    []FacetBucket `json:"service"`
	Operation  []FacetBucket `json:"operation"`
	HTTPMethod []FacetBucket `json:"httpMethod"`
	HTTPStatus []FacetBucket `json:"httpStatus"`
	Status     []FacetBucket `json:"status"`
}

type TrendBucket struct {
	TimeBucketMs int64  `json:"timeBucketMs"`
	Total        uint64 `json:"total"`
	Errors       uint64 `json:"errors"`
}

type Suggestion = filterutil.Suggestion

type SuggestResponse = filterutil.SuggestResponse

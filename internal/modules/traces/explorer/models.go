package explorer

import (
	"github.com/optikklabs/query/internal/infra/cursor"
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
	SpanCount      uint32   `json:"spanCount"`
	HasError       bool     `json:"hasError"`
	ErrorCount     uint32   `json:"errorCount"`
	ServiceSet     []string `json:"serviceSet,omitempty"`
	Truncated      bool     `json:"truncated,omitempty"`
}

type PageInfo struct {
	HasMore    bool   `json:"hasMore"`
	NextCursor string `json:"nextCursor,omitempty"`
	Limit      int    `json:"limit"`
}

type TraceCursor struct {
	StartNs uint64 `json:"s"`
	SpanID  string `json:"p"`
}

func (c TraceCursor) IsZero() bool { return c.SpanID == "" }

func (c TraceCursor) Encode() string {
	if c.IsZero() {
		return ""
	}
	return cursor.Encode(c)
}

func DecodeCursor(raw string) (TraceCursor, bool) {
	return cursor.Decode[TraceCursor](raw)
}

type FacetBucket struct {
	Value string `json:"value"`
	Count uint64 `json:"count"`
}

type Facets struct {
	Service    []FacetBucket `json:"service,omitempty"`
	Operation  []FacetBucket `json:"operation,omitempty"`
	HTTPMethod []FacetBucket `json:"httpMethod,omitempty"`
	HTTPStatus []FacetBucket `json:"httpStatus,omitempty"`
	Status     []FacetBucket `json:"status,omitempty"`
}

type TrendBucket struct {
	TimeBucket string `json:"timeBucket"`
	Total      uint64 `json:"total"`
	Errors     uint64 `json:"errors"`
}

// Suggestion is a type alias for the shared suggestion value+count pair.
// Kept as an alias so existing callers (e.g. wirefixtures) can still
// reference tracesexplorer.Suggestion without changes.
type Suggestion = filterutil.Suggestion

// SuggestResponse is a type alias for the shared suggest wire response.
type SuggestResponse = filterutil.SuggestResponse

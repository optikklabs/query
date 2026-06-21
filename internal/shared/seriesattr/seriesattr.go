// Package seriesattr holds canonical metrics_series attribute-path expressions
// for the APM (spanmetrics) dimensions that were formerly stored as fixed
// columns on the histogram rollup. The dimension now lives in the series
// metadata JSON and is resolved to a fingerprint set before the rollup join.
// Keys match the dimension names the histogram rollup MV formerly extracted.
package seriesattr

const (
	StatusCode     = "attributes.`status.code`::String"
	HTTPStatusCode = "attributes.`http.status_code`::String"
	SpanName       = "attributes.`span.name`::String"
	SpanKind       = "attributes.`span.kind`::String"
	Client         = "attributes.`client`::String"
	Server         = "attributes.`server`::String"
	// Le resolves a histogram bucket upper bound from a metrics_series row. The
	// "+Inf" overflow bucket casts to +inf, so isInf(le) selects the total count.
	Le = "attributes.`le`::Float64"
)

// StatusErrorPred tests the error status on the value carried out of a series
// CTE aliased `series` exposing a `status_code` column.
const StatusErrorPred = "(series.status_code = 'STATUS_CODE_ERROR' OR series.status_code = 'ERROR')"

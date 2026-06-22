// Package seriesattr holds canonical metrics_series attribute-path expressions
// for the APM (spanmetrics) dimensions that were formerly stored as fixed
// columns on the histogram rollup. The dimension now lives in the series
// metadata JSON and is resolved to a fingerprint set before the rollup join.
// Keys match the dimension names the histogram rollup MV formerly extracted.
package seriesattr

const (
	StatusCode     = "attributes.`status.code`::String"
	HTTPStatusCode = "attributes.`http.status_code`::String"
	HTTPRoute      = "attributes.`http.route`::String"
	SpanName       = "attributes.`span.name`::String"
	SpanKind       = "attributes.`span.kind`::String"
	DBSystem       = "attributes.`db.system`::String"
	Client         = "attributes.`client`::String"
	Server         = "attributes.`server`::String"
	// Le resolves a histogram bucket upper bound from a metrics_series row. The
	// "+Inf" overflow bucket casts to +inf, so isInf(le) selects the total count.
	Le = "attributes.`le`::Float64"
)

// StatusErrorPred tests the error status on the value carried out of a series
// CTE aliased `series` exposing a `status_code` column.
const StatusErrorPred = "(series.status_code = 'STATUS_CODE_ERROR' OR series.status_code = 'ERROR')"

// ServerKindPred restricts a metrics_series scan to inbound (SERVER) spans, so a
// service's RED request/error/latency reflects requests, not internal/client spans.
const ServerKindPred = "attributes.`span.kind`::String = 'SPAN_KIND_SERVER'"

// DBSpanPred restricts a metrics_series scan to database client spans, which
// carry a non-empty db.system dimension from the spanmetrics connector.
const DBSpanPred = "attributes.`db.system`::String != ''"

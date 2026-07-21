// Package seriesattr holds canonical metrics_series attribute-path expressions
// for the APM (spanmetrics) dimensions that were formerly stored as fixed
// columns on the histogram rollup. The dimension now lives in the series
// metadata JSON and is resolved to a fingerprint set before the rollup join.
// Keys match the dimension names the histogram rollup MV formerly extracted.
package seriesattr

const (
	StatusCode     = "attributes['status.code']"
	HTTPStatusCode = "attributes['http.status_code']"
	HTTPRoute      = "attributes['http.route']"
	SpanName       = "attributes['span.name']"
	SpanKind       = "attributes['span.kind']"
	DBSystem       = "attributes['db.system']"
	Client         = "attributes['client']"
	Server         = "attributes['server']"

	Le = "attributes.le::Float64"
)

const StatusErrorPred = "(series.status_code = 'STATUS_CODE_ERROR' OR series.status_code = 'ERROR')"

const DBSpanPred = "attributes['db.system'] != ''"

// Package seriesattr holds canonical metrics_series attribute-path expressions
// for the APM (spanmetrics) dimensions that were formerly stored as fixed
// columns on the histogram rollup. The dimension now lives in the series
// metadata JSON and is resolved to a fingerprint set before the rollup join.
// Keys match the dimension names the histogram rollup MV formerly extracted.
package seriesattr

const (
	StatusCode     = "attributes.status.code::String"
	HTTPStatusCode = "attributes.http.status_code::String"
	HTTPRoute      = "attributes.http.route::String"
	SpanName       = "attributes.span.name::String"
	SpanKind       = "attributes.span.kind::String"
	DBSystem       = "attributes.db.system::String"
	Client         = "attributes.client::String"
	Server         = "attributes.server::String"

	Le = "attributes.le::Float64"
)

const StatusErrorPred = "(series.status_code = 'STATUS_CODE_ERROR' OR series.status_code = 'ERROR')"

// EntryKindPred matches inbound "entry-point" spans. In OTel both SERVER and
// CONSUMER receive work (HTTP servers and messaging consumers), so RED metrics
// for a service are computed over both.
const EntryKindPred = "(attributes.span.kind::String IN ('SPAN_KIND_SERVER','SERVER','SPAN_KIND_CONSUMER','CONSUMER'))"

const DBSpanPred = "attributes.db.system::String != ''"

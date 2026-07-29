package dashboards

// Panel types the metrics widget builder can produce. The pre-builder,
// endpoint-backed panels (request, error-rate, latency, stat-card, …) were
// retired along with their renderers; no client can create or draw them.
var dashboardPanelTypes = map[string]struct{}{
	"metrics-timeseries": {}, "metrics-value": {}, "metrics-toplist": {}, "metrics-table": {},
}

func isValidPanelType(t string) bool {
	_, ok := dashboardPanelTypes[t]
	return ok
}

var metricsBuilderAggregations = map[string]struct{}{
	"avg": {}, "sum": {}, "min": {}, "max": {}, "count": {},
	"p50": {}, "p95": {}, "p99": {}, "rate": {},
}

func isValidBuilderAggregation(a string) bool {
	_, ok := metricsBuilderAggregations[a]
	return ok
}

var metricsBuilderOperators = map[string]struct{}{
	"eq": {}, "neq": {}, "in": {}, "not_in": {}, "wildcard": {},
}

func isValidBuilderOperator(op string) bool {
	_, ok := metricsBuilderOperators[op]
	return ok
}

// One per panel type, mirroring the builder's viz→variant mapping.
var dashboardLayoutVariants = map[string]struct{}{
	"standard-chart": {}, "kpi": {}, "ranking": {}, "detail-table": {},
}

func isValidLayoutVariant(v string) bool {
	_, ok := dashboardLayoutVariants[v]
	return ok
}

const maxWidgetsPerPage = 30

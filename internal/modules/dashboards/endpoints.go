package dashboards

var dashboardSafeEndpoints = map[string]struct{}{
	"/spans/red/fleet-totals":                   {},
	"/spans/red/apdex":                          {},
	"/spans/red/request-and-error-rate":         {},
	"/spans/red/status-timeseries":              {},
	"/spans/red/latency-percentiles-timeseries": {},
	"/spans/red/red-by-endpoint":                {},
	"/spans/red/top-endpoints":                  {},
	"/spans/red/top-db-queries":                 {},
	"/spans/red/services":                       {},
	"/errors/service-error-rate":                {},
	"/errors/error-volume":                      {},
	"/infrastructure/cpu/avg":                   {},
	"/infrastructure/cpu/by-instance":           {},
	"/infrastructure/memory/avg":                {},
	"/infrastructure/memory/by-instance":        {},
	"/saturation/database/latency/by-system":    {},
}

func isAllowedEndpoint(ep string) bool {
	_, ok := dashboardSafeEndpoints[ep]
	return ok
}

var dashboardPanelTypes = map[string]struct{}{
	"bar": {}, "db-systems-overview": {}, "error-rate": {},
	"exception-type-line": {}, "gauge": {}, "heatmap": {}, "latency": {},
	"latency-heatmap": {}, "latency-histogram": {}, "log-histogram": {},
	"pie": {}, "request": {}, "service-catalog": {}, "service-health-grid": {},
	"service-map": {}, "stat-card": {}, "stat-cards-grid": {}, "stat-summary": {},
	"table": {}, "trace-waterfall": {},
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

var dashboardLayoutVariants = map[string]struct{}{
	"kpi": {}, "summary": {}, "standard-chart": {}, "wide-chart": {},
	"ranking": {}, "summary-table": {}, "detail-table": {}, "hero": {},
	"hero-map": {}, "hero-detail": {}, "compact": {}, "wide-compact": {},
}

func isValidLayoutVariant(v string) bool {
	_, ok := dashboardLayoutVariants[v]
	return ok
}

const maxWidgetsPerPage = 30

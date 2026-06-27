package dashboards

// This file is the single backend source of truth for what a widget may
// reference. A widget never carries raw SQL; it points at a curated, already
// optimized GET endpoint plus query params. Anything outside these sets is
// rejected at create/update time.

// dashboardSafeEndpoints is the allowlist of time-bounded, team-scoped module
// endpoints a widget query may target. All are GET endpoints that already apply
// PREWHERE team_id + WHERE timestamp BETWEEN @start AND @end.
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

// dashboardPanelTypes mirrors web DASHBOARD_PANEL_TYPES; a widget's panel_type
// must be one of these so the frontend always has a registered renderer. The
// metrics-* types back the SigNoz-style query-builder widgets.
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

// metricsBuilderAggregations mirrors web MetricAggregation and the engine's
// validAggregations (query/internal/modules/metrics/filter).
var metricsBuilderAggregations = map[string]struct{}{
	"avg": {}, "sum": {}, "min": {}, "max": {}, "count": {},
	"p50": {}, "p95": {}, "p99": {}, "rate": {},
}

func isValidBuilderAggregation(a string) bool {
	_, ok := metricsBuilderAggregations[a]
	return ok
}

// metricsBuilderOperators mirrors web MetricFilterOperator.
var metricsBuilderOperators = map[string]struct{}{
	"eq": {}, "neq": {}, "in": {}, "not_in": {}, "wildcard": {},
}

func isValidBuilderOperator(op string) bool {
	_, ok := metricsBuilderOperators[op]
	return ok
}

// dashboardLayoutVariants mirrors web DASHBOARD_LAYOUT_VARIANTS.
var dashboardLayoutVariants = map[string]struct{}{
	"kpi": {}, "summary": {}, "standard-chart": {}, "wide-chart": {},
	"ranking": {}, "summary-table": {}, "detail-table": {}, "hero": {},
	"hero-map": {}, "hero-detail": {}, "compact": {}, "wide-compact": {},
}

func isValidLayoutVariant(v string) bool {
	_, ok := dashboardLayoutVariants[v]
	return ok
}

// maxWidgetsPerPage caps fan-out: bounds worst-case concurrent panel queries.
const maxWidgetsPerPage = 30

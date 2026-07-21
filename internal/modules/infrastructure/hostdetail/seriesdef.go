package hostdetail

import "github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"

// Datapoint-attribute accessors on metrics_series.attributes (JSON).
// Keys match what the OTel hostmetrics scrapers actually emit.
const (
	attrState      = "coalesce(attributes['state'], '')"
	attrDevice     = "coalesce(attributes['device'], '')"
	attrDirection  = "coalesce(attributes['direction'], '')"
	attrMountpoint = "coalesce(attributes['mountpoint'], '')"
)

type aggKind int

const (
	// aggGauge averages datapoint values per display bucket.
	aggGauge aggKind = iota
	// aggRate converts counters to per-second rates per display bucket.
	aggRate
)

// SeriesDef describes one chartable metric group on the host detail page.
type SeriesDef struct {
	ID          string
	MetricNames []string
	// LabelSQL evaluates against metrics_series rows to name each series.
	LabelSQL string
	Agg      aggKind
	// Scale multiplies values after aggregation (100 for 0..1 fractions).
	Scale float64
}

var seriesDefs = []SeriesDef{
	{
		ID:          "cpu",
		MetricNames: []string{infraconsts.MetricSystemCPUUtilization},
		LabelSQL:    "if(" + attrState + " = '', 'cpu', " + attrState + ")",
		Agg:         aggGauge,
		Scale:       infraconsts.PercentageMultiplier,
	},
	{
		ID: "load",
		MetricNames: []string{
			infraconsts.MetricSystemCPULoadAvg1m,
			infraconsts.MetricSystemCPULoadAvg5m,
			infraconsts.MetricSystemCPULoadAvg15m,
		},
		LabelSQL: "multiIf(metric_name = '" + infraconsts.MetricSystemCPULoadAvg1m + "', '1m', " +
			"metric_name = '" + infraconsts.MetricSystemCPULoadAvg5m + "', '5m', '15m')",
		Agg:   aggGauge,
		Scale: 1,
	},
	{
		ID:          "memory",
		MetricNames: []string{infraconsts.MetricSystemMemoryUsage},
		LabelSQL:    "if(" + attrState + " = '', 'memory', " + attrState + ")",
		Agg:         aggGauge,
		Scale:       1,
	},
	{
		ID:          "disk_io",
		MetricNames: []string{infraconsts.MetricSystemDiskIO},
		LabelSQL:    deviceDirectionLabel("disk"),
		Agg:         aggRate,
		Scale:       1,
	},
	{
		ID:          "filesystem",
		MetricNames: []string{infraconsts.MetricSystemFilesystemUtil},
		LabelSQL:    "if(" + attrMountpoint + " = '', 'filesystem', " + attrMountpoint + ")",
		Agg:         aggGauge,
		Scale:       infraconsts.PercentageMultiplier,
	},
	{
		ID:          "network_io",
		MetricNames: []string{infraconsts.MetricSystemNetworkIO},
		LabelSQL:    deviceDirectionLabel("network"),
		Agg:         aggRate,
		Scale:       1,
	},
	{
		ID: "network_errors",
		MetricNames: []string{
			infraconsts.MetricSystemNetworkErrors,
			infraconsts.MetricSystemNetworkDropped,
		},
		LabelSQL: "trim(concat(if(metric_name = '" + infraconsts.MetricSystemNetworkErrors + "', 'errors', 'dropped'), " +
			"' ', " + attrDirection + "))",
		Agg:   aggRate,
		Scale: 1,
	},
}

// deviceDirectionLabel names IO series "<device> <direction>" with a fallback.
func deviceDirectionLabel(fallback string) string {
	expr := "trim(concat(" + attrDevice + ", ' ', " + attrDirection + "))"
	return "if(" + expr + " = '', '" + fallback + "', " + expr + ")"
}

var seriesDefByID = func() map[string]SeriesDef {
	m := make(map[string]SeriesDef, len(seriesDefs))
	for _, d := range seriesDefs {
		m[d.ID] = d
	}
	return m
}()

// SeriesDefFor resolves an API metric id to its definition.
func SeriesDefFor(id string) (SeriesDef, bool) {
	d, ok := seriesDefByID[id]
	return d, ok
}

// allSeriesMetricNames is the union of metric names across all groups,
// used to detect which chart groups have data for a host.
var allSeriesMetricNames = func() []string {
	var names []string
	for _, d := range seriesDefs {
		names = append(names, d.MetricNames...)
	}
	return names
}()

// groupsForMetricNames maps present metric names to available group ids,
// preserving seriesDefs order.
func groupsForMetricNames(present []string) []string {
	set := make(map[string]struct{}, len(present))
	for _, n := range present {
		set[n] = struct{}{}
	}
	var groups []string
	for _, d := range seriesDefs {
		for _, n := range d.MetricNames {
			if _, ok := set[n]; ok {
				groups = append(groups, d.ID)
				break
			}
		}
	}
	return groups
}

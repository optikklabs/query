package containerdetail

import "github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"

// Datapoint-attribute accessors on metrics_series.attributes (JSON).
// Keys match what the OTel kubeletstats receiver and JVM SDKs emit.
const (
	attrDirection     = "coalesce(attributes.`direction`::String, '')"
	attrInterface     = "coalesce(attributes.`interface`::String, '')"
	attrJVMMemoryType = "coalesce(attributes.`jvm.memory.type`::String, '')"
)

// containerLabel names series by the k8s container column, falling back to
// the pod itself for pod-level metrics that carry no container.
const containerLabel = "if(container = '', 'pod', container)"

type aggKind int

const (
	// aggGauge averages datapoint values per display bucket.
	aggGauge aggKind = iota
	// aggRate converts counters to per-second rates per display bucket.
	aggRate
)

// SeriesDef describes one chartable metric group on the container detail page.
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
		ID: "cpu",
		MetricNames: []string{
			infraconsts.MetricK8SPodCPUUtilization,
			infraconsts.MetricContainerCPUUtilization,
		},
		LabelSQL: containerLabel,
		Agg:      aggGauge,
		Scale:    infraconsts.PercentageMultiplier,
	},
	{
		ID: "memory",
		MetricNames: []string{
			infraconsts.MetricK8SPodMemoryUsage,
			infraconsts.MetricK8SPodMemoryWorkingSet,
			infraconsts.MetricContainerMemoryUsage,
		},
		LabelSQL: "trim(concat(" + containerLabel + ", ' ', " +
			"if(metric_name = '" + infraconsts.MetricK8SPodMemoryWorkingSet + "', 'working set', 'usage')))",
		Agg:   aggGauge,
		Scale: 1,
	},
	{
		ID:          "network_io",
		MetricNames: []string{infraconsts.MetricK8SPodNetworkIO},
		LabelSQL:    interfaceDirectionLabel("network"),
		Agg:         aggRate,
		Scale:       1,
	},
	{
		ID:          "network_errors",
		MetricNames: []string{infraconsts.MetricK8SPodNetworkErrors},
		LabelSQL:    interfaceDirectionLabel("errors"),
		Agg:         aggRate,
		Scale:       1,
	},
	{
		ID: "filesystem",
		MetricNames: []string{
			infraconsts.MetricK8SPodFilesystemUsage,
			infraconsts.MetricK8SPodFilesystemCapacity,
			infraconsts.MetricK8SPodFilesystemAvailable,
		},
		LabelSQL: "multiIf(metric_name = '" + infraconsts.MetricK8SPodFilesystemUsage + "', 'used', " +
			"metric_name = '" + infraconsts.MetricK8SPodFilesystemCapacity + "', 'capacity', 'available')",
		Agg:   aggGauge,
		Scale: 1,
	},
	{
		ID:          "restarts",
		MetricNames: []string{infraconsts.MetricK8SContainerRestarts},
		LabelSQL:    containerLabel,
		Agg:         aggGauge,
		Scale:       1,
	},
	{
		ID:          "jvm_memory",
		MetricNames: []string{infraconsts.MetricJVMMemoryUsed},
		LabelSQL:    "if(" + attrJVMMemoryType + " = '', 'memory', " + attrJVMMemoryType + ")",
		Agg:         aggGauge,
		Scale:       1,
	},
}

// interfaceDirectionLabel names IO series "<interface> <direction>" with a
// fallback for datapoints missing both attributes.
func interfaceDirectionLabel(fallback string) string {
	expr := "trim(concat(" + attrInterface + ", ' ', " + attrDirection + "))"
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
// used to detect which chart groups have data for a pod.
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

package containerdetail

import (
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
)

// Datapoint-attribute accessors on metrics_series.attributes. Map lookups
// default to the empty string. Keys match kubeletstats and JVM SDK output.
const (
	attrDirection     = "attributes['direction']"
	attrInterface     = "attributes['interface']"
	attrJVMMemoryType = "attributes['jvm.memory.type']"
)

// containerLabel names series by the k8s container column, falling back to
// the pod itself for pod-level metrics that carry no container.
const containerLabel = "if(container = '', 'pod', container)"

// catalog is the container detail page's chart groups, in display order.
var catalog = seriesgroup.NewCatalog(seriesDefs)

var seriesDefs = []seriesgroup.Def{
	{
		ID: "cpu",
		MetricNames: []string{
			infraconsts.MetricK8SPodCPUUtilization,
			infraconsts.MetricContainerCPUUtilization,
		},
		LabelSQL: containerLabel,
		Agg:      seriesgroup.Gauge,
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
		Agg:   seriesgroup.Gauge,
		Scale: 1,
	},
	{
		ID:          "network_io",
		MetricNames: []string{infraconsts.MetricK8SPodNetworkIO},
		LabelSQL:    interfaceDirectionLabel("network"),
		Agg:         seriesgroup.Rate,
		Scale:       1,
	},
	{
		ID:          "network_errors",
		MetricNames: []string{infraconsts.MetricK8SPodNetworkErrors},
		LabelSQL:    interfaceDirectionLabel("errors"),
		Agg:         seriesgroup.Rate,
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
		Agg:   seriesgroup.Gauge,
		Scale: 1,
	},
	{
		ID:          "restarts",
		MetricNames: []string{infraconsts.MetricK8SContainerRestarts},
		LabelSQL:    containerLabel,
		Agg:         seriesgroup.Gauge,
		Scale:       1,
	},
	{
		ID:          "jvm_memory",
		MetricNames: []string{infraconsts.MetricJVMMemoryUsed},
		LabelSQL:    "if(" + attrJVMMemoryType + " = '', 'memory', " + attrJVMMemoryType + ")",
		Agg:         seriesgroup.Gauge,
		Scale:       1,
	},
}

// interfaceDirectionLabel names IO series "<interface> <direction>" with a
// fallback for datapoints missing both attributes.
func interfaceDirectionLabel(fallback string) string {
	expr := "trim(concat(" + attrInterface + ", ' ', " + attrDirection + "))"
	return "if(" + expr + " = '', '" + fallback + "', " + expr + ")"
}

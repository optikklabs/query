package hostdetail

import (
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
)

// Datapoint-attribute accessors on metrics_series.attributes. Map lookups
// default to the empty string. Keys match what hostmetrics scrapers emit.
const (
	attrState      = "attributes['state']"
	attrDevice     = "attributes['device']"
	attrDirection  = "attributes['direction']"
	attrMountpoint = "attributes['mountpoint']"
)

// catalog is the host detail page's chart groups, in display order.
var catalog = seriesgroup.NewCatalog(seriesDefs)

var seriesDefs = []seriesgroup.Def{
	{
		ID:          "cpu",
		MetricNames: []string{infraconsts.MetricSystemCPUUtilization},
		LabelSQL:    "if(" + attrState + " = '', 'cpu', " + attrState + ")",
		Agg:         seriesgroup.Gauge,
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
		Agg:   seriesgroup.Gauge,
		Scale: 1,
	},
	{
		ID:          "memory",
		MetricNames: []string{infraconsts.MetricSystemMemoryUsage},
		LabelSQL:    "if(" + attrState + " = '', 'memory', " + attrState + ")",
		Agg:         seriesgroup.Gauge,
		Scale:       1,
	},
	{
		ID:          "disk_io",
		MetricNames: []string{infraconsts.MetricSystemDiskIO},
		LabelSQL:    deviceDirectionLabel("disk"),
		Agg:         seriesgroup.Rate,
		Scale:       1,
	},
	{
		ID:          "filesystem",
		MetricNames: []string{infraconsts.MetricSystemFilesystemUtil},
		LabelSQL:    "if(" + attrMountpoint + " = '', 'filesystem', " + attrMountpoint + ")",
		Agg:         seriesgroup.Gauge,
		Scale:       infraconsts.PercentageMultiplier,
	},
	{
		ID:          "network_io",
		MetricNames: []string{infraconsts.MetricSystemNetworkIO},
		LabelSQL:    deviceDirectionLabel("network"),
		Agg:         seriesgroup.Rate,
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
		Agg:   seriesgroup.Rate,
		Scale: 1,
	},
}

// deviceDirectionLabel names IO series "<device> <direction>" with a fallback.
func deviceDirectionLabel(fallback string) string {
	expr := "trim(concat(" + attrDevice + ", ' ', " + attrDirection + "))"
	return "if(" + expr + " = '', '" + fallback + "', " + expr + ")"
}

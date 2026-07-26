// Package seriesdefs declares the chart-group catalogs for the resource
// detail pages. A catalog is a definition, not a layer: the repository reads
// it to know which metric names to scan for, and the service reads it to
// resolve an API metric id and to list a resource's available groups. It sits
// beside seriesgroup and infraconsts as shared infrastructure vocabulary
// rather than inside either layer.
package seriesdefs

import (
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
)

// Datapoint-attribute accessors on metrics_series.attributes. Map lookups
// default to the empty string. Keys match what hostmetrics scrapers emit.
// AttrState and AttrMountpoint are exported because the host KPI query groups
// by them directly.
const (
	AttrState      = "attributes['state']"
	AttrMountpoint = "attributes['mountpoint']"

	attrDevice    = "attributes['device']"
	attrDirection = "attributes['direction']"
)

// Host is the host detail page's chart groups, in display order.
var Host = seriesgroup.NewCatalog(hostDefs)

var hostDefs = []seriesgroup.Def{
	{
		ID:          "cpu",
		MetricNames: []string{infraconsts.MetricSystemCPUUtilization},
		LabelSQL:    "if(" + AttrState + " = '', 'cpu', " + AttrState + ")",
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
		LabelSQL:    "if(" + AttrState + " = '', 'memory', " + AttrState + ")",
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
		LabelSQL:    "if(" + AttrMountpoint + " = '', 'filesystem', " + AttrMountpoint + ")",
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

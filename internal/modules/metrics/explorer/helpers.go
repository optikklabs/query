package explorer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/metrics/filter"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

func normalizeMetricType(t string) string {
	normalized := strings.ToLower(t)
	switch normalized {
	case "gauge", "histogram", "summary":
		return normalized
	case "sum":
		return "counter"
	case "exponentialhistogram":
		return "exponential_histogram"
	default:
		return ""
	}
}

func applyAggregation(rows []timeseriesPointDTO, aggregation string, startMs, endMs int64, step string, cumulative, histogram bool) []TimeseriesPoint {
	bucketSec := float64(filter.BucketDurationSeconds(startMs, endMs, step))
	out := make([]TimeseriesPoint, len(rows))
	for i, row := range rows {
		out[i] = TimeseriesPoint{
			TimestampMs: row.BucketAt.UnixMilli(),
			Value:       computeValue(row, aggregation, bucketSec, cumulative, histogram),
		}
	}
	return out
}

func computeValue(row timeseriesPointDTO, agg string, bucketSec float64, cumulative, histogram bool) float64 {
	switch {
	case cumulative:
		return cumulativeValue(row, agg, bucketSec)
	case histogram:
		return histogramValue(row, agg, bucketSec)
	default:
		return deltaValue(row, agg, bucketSec)
	}
}

func cumulativeValue(row timeseriesPointDTO, agg string, bucketSec float64) float64 {
	if agg == "rate" {
		return row.Sum / bucketSec
	}
	return row.Sum
}

func histogramValue(row timeseriesPointDTO, agg string, bucketSec float64) float64 {
	switch agg {
	case "p50", "p95", "p99":
		return quantileFor(row.Quantiles, agg)
	case "sum":
		return row.HistSum
	case "count":
		return float64(row.HistCount)
	case "rate":
		return float64(row.HistCount) / bucketSec
	case "min":
		return row.Min
	case "max":
		return row.Max
	case "avg":
		if row.HistCount > 0 {
			return row.HistSum / float64(row.HistCount)
		}
		return 0
	default:
		return 0
	}
}

func deltaValue(row timeseriesPointDTO, agg string, bucketSec float64) float64 {
	switch agg {
	case "sum":
		return row.Sum
	case "min":
		return row.Min
	case "max":
		return row.Max
	case "count":
		return float64(row.Count)
	case "rate":
		return row.Sum / bucketSec
	default:
		if row.Count > 0 {
			return row.Sum / float64(row.Count)
		}
		return 0
	}
}

func quantileFor(qs []float64, aggregation string) float64 {
	idx := map[string]int{"p50": 0, "p95": 1, "p99": 2}[aggregation]
	if idx < len(qs) {
		return qs[idx]
	}
	return 0
}

func convertFEQuery(tenantID, startMs, endMs int64, step string, query FEMetricQuery) filter.Filters {
	tags := make([]filter.TagFilter, 0, len(query.Where))
	for _, item := range query.Where {
		tags = append(tags, filter.TagFilter{
			Key:      item.Key,
			Operator: filterutil.MapOperator(item.Operator),
			Values:   filterutil.ExtractValues(item.Value),
		})
	}

	return filter.Filters{
		TenantID:    tenantID,
		StartMs:     startMs,
		EndMs:       endMs,
		MetricName:  query.MetricName,
		Aggregation: query.Aggregation,
		Step:        step,
		GroupBy:     query.GroupBy,
		Tags:        tags,
	}
}

func resolveMetricKind(kind metricNameDTO) (cumulative, histogram bool, err error) {
	if kind.Variants != 1 {
		return false, false, fmt.Errorf("metric name has incompatible series types")
	}

	switch normalizeMetricType(kind.MetricType) {
	case "summary":
		return false, false, fmt.Errorf("summary metrics are not safely aggregatable")
	case "histogram", "exponential_histogram":
		if kind.Temporality == "Cumulative" {
			return false, false, fmt.Errorf("cumulative distributions are not safely aggregatable")
		}
		return false, true, nil
	case "counter":
		return kind.Temporality == "Cumulative" && kind.IsMonotonic, false, nil
	case "gauge":
		return false, false, nil
	default:
		return false, false, fmt.Errorf("unsupported metric type %q", kind.MetricType)
	}
}

func validateAggregationForMode(aggregation string, cumulative, histogram bool) error {
	if cumulative && aggregation != "sum" && aggregation != "rate" {
		return fmt.Errorf("%s is not supported for cumulative counters", aggregation)
	}
	if histogram && (aggregation == "min" || aggregation == "max") {
		return fmt.Errorf("%s is not retained for distribution metrics", aggregation)
	}
	return nil
}

func shouldZeroFill(metricType, aggregation string, cumulative bool) bool {
	return strings.EqualFold(metricType, "sum") || cumulative || aggregation == "count" || aggregation == "rate"
}

func mapPointsToAxis(points []TimeseriesPoint, tsIndex map[int64]int, length int) []*float64 {
	values := make([]*float64, length)
	for _, p := range points {
		if idx, ok := tsIndex[p.TimestampMs]; ok {
			v := p.Value
			values[idx] = &v
		}
	}
	return values
}

func buildColumnarResult(points []TimeseriesPoint, startMs, endMs int64, step string, fillZero bool) FEQueryResult {
	bucketSec := filter.BucketDurationSeconds(startMs, endMs, step)
	timestamps := timebucket.BuildDenseTimestamps(startMs, endMs, bucketSec)
	if len(timestamps) == 0 {
		return FEQueryResult{Timestamps: []int64{}, Series: []FESeries{}}
	}

	tsIndex := make(map[int64]int, len(timestamps))
	for i, ts := range timestamps {
		tsIndex[ts] = i
	}

	values := mapPointsToAxis(points, tsIndex, len(timestamps))
	if fillZero {
		timebucket.ZeroFillGaps(values)
	}

	return FEQueryResult{
		Timestamps: timestamps,
		Series:     []FESeries{{Tags: map[string]string{}, Values: values}},
	}
}

func buildGroupedColumnarResult(
	rows []timeseriesPointDTO,
	points []TimeseriesPoint,
	groupKeys []string,
	startMs, endMs int64,
	step string,
	fillZero bool,
) FEQueryResult {
	if len(groupKeys) == 0 {
		return buildColumnarResult(points, startMs, endMs, step, fillZero)
	}

	bucketSec := filter.BucketDurationSeconds(startMs, endMs, step)
	timestamps := timebucket.BuildDenseTimestamps(startMs, endMs, bucketSec)
	if len(timestamps) == 0 || len(rows) == 0 {
		return FEQueryResult{Timestamps: timestamps, Series: []FESeries{}}
	}

	tsIndex := make(map[int64]int, len(timestamps))
	for i, ts := range timestamps {
		tsIndex[ts] = i
	}

	groups := make(map[string]*pointGroup)
	for i, row := range rows {
		key := groupIdentity(row.GroupValues)
		group := groups[key]
		if group == nil {
			group = &pointGroup{tags: groupTags(groupKeys, row.GroupValues)}
			groups[key] = group
		}
		group.points = append(group.points, points[i])
	}

	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	series := make([]FESeries, 0, len(keys))
	for _, key := range keys {
		group := groups[key]
		values := mapPointsToAxis(group.points, tsIndex, len(timestamps))
		if fillZero {
			timebucket.ZeroFillGaps(values)
		}
		series = append(series, FESeries{Tags: group.tags, Values: values})
	}
	return FEQueryResult{Timestamps: timestamps, Series: series}
}

func groupIdentity(values []string) string {
	var b strings.Builder
	for _, value := range values {
		fmt.Fprintf(&b, "%d:%s|", len(value), value)
	}
	return b.String()
}

func groupTags(keys, values []string) map[string]string {
	tags := make(map[string]string, len(keys))
	for i, key := range keys {
		if i < len(values) {
			tags[key] = values[i]
		} else {
			tags[key] = ""
		}
	}
	return tags
}

type pointGroup struct {
	tags   map[string]string
	points []TimeseriesPoint
}

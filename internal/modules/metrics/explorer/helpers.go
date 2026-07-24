package explorer

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/metrics/filter"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

// normalizeMetricType maps ClickHouse/OTLP metric type names to the lowercase
// values the frontend Zod schema expects.
func normalizeMetricType(t string) string {
	switch strings.ToLower(t) {
	case "gauge":
		return "gauge"
	case "sum":
		return "counter"
	case "histogram":
		return "histogram"
	case "summary":
		return "summary"
	default:
		return "gauge"
	}
}

func applyAggregation(rows []timeseriesPointDTO, aggregation string, startMs, endMs int64, step string, cumulative, histogram bool) []TimeseriesPoint {
	bucketSec := float64(filter.BucketDurationSeconds(startMs, endMs, step))
	out := make([]TimeseriesPoint, len(rows))
	for i, row := range rows {
		out[i] = TimeseriesPoint{
			Timestamp: timebucket.FormatDisplayBucket(row.BucketAt),
			Value:     computeValue(row, aggregation, bucketSec, cumulative, histogram),
		}
	}
	return out
}

// computeValue dispatches to the right value-extraction function based on the
// metric kind flags.
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

// cumulativeValue extracts the value for a cumulative counter row.
// rate scales the per-bucket increase by seconds; everything else returns the
// increase directly.
func cumulativeValue(row timeseriesPointDTO, agg string, bucketSec float64) float64 {
	if agg == "rate" {
		return row.Sum / bucketSec
	}
	return row.Sum
}

// histogramValue extracts the value for a histogram-type row.
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

// deltaValue extracts the value for a plain delta / gauge row.
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
	default: // avg
		if row.Count > 0 {
			return row.Sum / float64(row.Count)
		}
		return 0
	}
}

// isPercentile returns true for percentile aggregation names (p50, p95, p99).
func isPercentile(aggregation string) bool {
	switch aggregation {
	case "p50", "p95", "p99":
		return true
	}
	return false
}

func quantileFor(qs []float64, aggregation string) float64 {
	idx := map[string]int{"p50": 0, "p95": 1, "p99": 2}[aggregation]
	if idx < len(qs) {
		return qs[idx]
	}
	return 0
}

func convertFEQuery(tenantID, startMs, endMs int64, step string, feq FEMetricQuery) filter.Filters {
	tags := make([]filter.TagFilter, 0, len(feq.Where))
	for _, w := range feq.Where {
		tags = append(tags, filter.TagFilter{
			Key:      w.Key,
			Operator: filterutil.MapOperator(w.Operator),
			Values:   filterutil.ExtractValues(w.Value),
		})
	}
	return filter.Filters{
		TenantID:    tenantID,
		StartMs:     startMs,
		EndMs:       endMs,
		MetricName:  feq.MetricName,
		Aggregation: feq.Aggregation,
		Step:        step,
		GroupBy:     feq.GroupBy,
		Tags:        tags,
	}
}

// resolveSeriesFlags derives cumulative and histogram booleans from the raw
// metricKindDTO returned by the repository. If kind is nil (no series found),
// both default to false.
func resolveSeriesFlags(kind *metricKindDTO) (cumulative, histogram bool) {
	if kind == nil {
		return false, false
	}
	cumulative = kind.Temporality == "Cumulative" && kind.IsMonotonic
	histogram = strings.EqualFold(kind.MetricType, "histogram")
	return cumulative, histogram
}

// metricTypeFrom extracts the raw OTLP metric type string ("Sum", "Gauge",
// "Histogram", "Summary") from the DTO, returning "" if kind is nil.
func metricTypeFrom(kind *metricKindDTO) string {
	if kind == nil {
		return ""
	}
	return kind.MetricType
}

// shouldZeroFill returns true when empty time buckets should be filled with 0
// rather than nil. Counters and sum-typed metrics represent accumulated values
// where absence means "no events" (= 0). Gauges represent instantaneous
// measurements where absence means "unknown" (= nil / break in line).
func shouldZeroFill(metricType, aggregation string, cumulative bool) bool {
	mt := strings.ToLower(metricType)
	switch {
	case mt == "sum" || cumulative:
		return true // counter/sum → 0 in gaps
	case aggregation == "count" || aggregation == "rate":
		return true // count/rate on any type → 0 in gaps
	default:
		return false // gauge, summary, histogram-percentile → nil
	}
}

// mapPointsToAxis places each aggregated point into the matching dense-axis
// slot, returning a sparse values slice (nil = no data).
func mapPointsToAxis(points []TimeseriesPoint, tsIndex map[int64]int, length int) []*float64 {
	values := make([]*float64, length)
	for _, p := range points {
		ms := parseTimestampMs(p.Timestamp)
		if idx, ok := tsIndex[ms]; ok {
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

func parseTimestampMs(ts string) int64 {
	t, err := time.Parse("2006-01-02 15:04:05", ts)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

type pointGroup struct {
	tags   map[string]string
	points []TimeseriesPoint
}

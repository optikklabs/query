package explorer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/metrics/filter"
	"golang.org/x/sync/errgroup"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListMetricNames(ctx context.Context, tenantID, startMs, endMs int64, search string) ([]MetricNameResult, error) {
	rows, err := s.repo.ListMetricNames(ctx, tenantID, startMs, endMs, search)
	if err != nil {
		return nil, err
	}
	out := make([]MetricNameResult, len(rows))
	for i, row := range rows {
		out[i] = MetricNameResult{
			MetricName:  row.MetricName,
			MetricType:  normalizeMetricType(row.MetricType),
			Unit:        row.Unit,
			Description: row.Description,
		}
	}
	return out, nil
}

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

func (s *Service) ListTagKeys(ctx context.Context, tenantID, startMs, endMs int64, metricName string) ([]TagKeyResult, error) {
	rows, err := s.repo.ListAttributeTagKeys(ctx, tenantID, startMs, endMs, metricName)
	if err != nil {
		return nil, err
	}

	staticKeys := []TagKeyResult{
		{TagKey: "service"},
		{TagKey: "host"},
		{TagKey: "environment"},
		{TagKey: "k8s_namespace"},
	}

	seen := make(map[string]bool)
	var out []TagKeyResult
	for _, sk := range staticKeys {
		seen[sk.TagKey] = true
		out = append(out, sk)
	}
	for _, row := range rows {
		if !seen[row.TagKey] {
			seen[row.TagKey] = true
			out = append(out, TagKeyResult{TagKey: row.TagKey})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].TagKey < out[j].TagKey
	})

	return out, nil
}

func (s *Service) ListTagValues(ctx context.Context, tenantID, startMs, endMs int64, metricName, tagKey string) ([]TagValueResult, error) {
	var rows []tagValueDTO
	var err error
	if canonical := filter.Canonical(tagKey); canonical != "" {
		rows, err = s.repo.ListResourceTagValues(ctx, tenantID, startMs, endMs, metricName, canonical)
	} else {
		rows, err = s.repo.ListAttributeTagValues(ctx, tenantID, startMs, endMs, metricName, tagKey)
	}
	if err != nil {
		return nil, err
	}
	out := make([]TagValueResult, len(rows))
	for i, row := range rows {
		out[i] = TagValueResult{TagValue: row.TagValue, Count: row.Count}
	}
	return out, nil
}

func (s *Service) ListTags(ctx context.Context, tenantID, startMs, endMs int64, metricName, tagKey string) ([]FETagEntry, error) {
	if tagKey != "" {
		values, err := s.ListTagValues(ctx, tenantID, startMs, endMs, metricName, tagKey)
		if err != nil {
			return nil, err
		}
		vals := make([]string, len(values))
		for i, v := range values {
			vals[i] = v.TagValue
		}
		return []FETagEntry{{Key: tagKey, Values: vals}}, nil
	}

	keys, err := s.ListTagKeys(ctx, tenantID, startMs, endMs, metricName)
	if err != nil {
		return nil, err
	}

	keyNames := make([]string, len(keys))
	for i, k := range keys {
		keyNames[i] = k.TagKey
	}

	rows, err := s.repo.ListTagValuesForKeys(ctx, tenantID, startMs, endMs, metricName, keyNames)
	if err != nil {
		return nil, err
	}

	valuesByKey := make(map[string][]string, len(keys))
	for _, row := range rows {
		valuesByKey[row.TagKey] = append(valuesByKey[row.TagKey], row.TagValue)
	}

	tags := make([]FETagEntry, len(keys))
	for i, k := range keys {
		vals := valuesByKey[k.TagKey]
		if vals == nil {

			vals = []string{}
		}
		tags[i] = FETagEntry{Key: k.TagKey, Values: vals}
	}
	return tags, nil
}

func (s *Service) Query(ctx context.Context, tenantID int64, req FEQueryRequest) (*FEQueryResponse, error) {
	type preparedQuery struct {
		request FEMetricQuery
		filter  filter.Filters
		result  FEQueryResult
	}
	prepared := make([]preparedQuery, len(req.Queries))
	metricNames := make([]string, 0, len(req.Queries))
	seenMetrics := make(map[string]struct{}, len(req.Queries))
	for i, feq := range req.Queries {
		f := convertFEQuery(tenantID, req.StartTime, req.EndTime, req.Step, feq)
		if err := f.Validate(); err != nil {
			return nil, fmt.Errorf("query %q: %w", feq.ID, err)
		}
		prepared[i] = preparedQuery{request: feq, filter: f}
		if _, seen := seenMetrics[f.MetricName]; !seen {
			seenMetrics[f.MetricName] = struct{}{}
			metricNames = append(metricNames, f.MetricName)
		}
	}

	kinds, err := s.repo.ResolveSeriesKinds(ctx, tenantID, req.StartTime, req.EndTime, metricNames)
	if err != nil {
		return nil, fmt.Errorf("resolve metric types: %w", err)
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(4)
	for i := range prepared {
		i := i
		group.Go(func() error {
			query := &prepared[i]
			kind, found := kinds[query.filter.MetricName]
			var kindPtr *metricKindDTO
			if found {
				kindPtr = &kind
			}
			query.filter.Cumulative, query.filter.Histogram = resolveSeriesFlags(kindPtr)
			metricType := metricTypeFrom(kindPtr)
			fillZero := shouldZeroFill(
				metricType, query.filter.Aggregation, query.filter.Cumulative,
			)

			rows, err := s.repo.QueryRollupSeries(groupCtx, query.filter)
			if err != nil {
				return fmt.Errorf("query %q: %w", query.request.ID, err)
			}

			points := applyAggregation(rows, query.filter.Aggregation, query.filter.StartMs, query.filter.EndMs, query.filter.Step, query.filter.Cumulative, query.filter.Histogram)
			query.result = buildGroupedColumnarResult(
				rows, points, query.request.GroupBy,
				query.filter.StartMs, query.filter.EndMs, query.filter.Step, fillZero,
			)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	results := make(map[string]FEQueryResult, len(prepared))
	for _, query := range prepared {
		results[query.request.ID] = query.result
	}

	return &FEQueryResponse{Results: results}, nil
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
			Operator: mapOperator(w.Operator),
			Values:   extractValues(w.Value),
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

func mapOperator(op string) string {
	switch op {
	case "eq":
		return "="
	case "neq":
		return "!="
	case "in":
		return "IN"
	case "not_in":
		return "NOT IN"
	default:
		return op
	}
}

func extractValues(v any) []string {
	switch val := v.(type) {
	case string:
		return []string{val}
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return val
	default:
		if s := fmt.Sprint(v); s != "" {
			return []string{s}
		}
		return nil
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

// buildDenseTimestamps generates an ordered slice of bucket-aligned timestamps
// covering [startMs, endMs].
func buildDenseTimestamps(startMs, endMs int64, step string) []int64 {
	bucketSec := filter.BucketDurationSeconds(startMs, endMs, step)
	flooredStart := timebucket.FloorMsToBucket(startMs, bucketSec)
	bucketMs := bucketSec * 1000

	var ts []int64
	for t := flooredStart; t <= endMs; t += bucketMs {
		ts = append(ts, t)
	}
	return ts
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

// zeroFillGaps replaces nil entries in values with a pointer to 0.0.
func zeroFillGaps(values []*float64) {
	zero := 0.0
	for i, v := range values {
		if v == nil {
			values[i] = &zero
		}
	}
}

func buildColumnarResult(points []TimeseriesPoint, startMs, endMs int64, step string, fillZero bool) FEQueryResult {
	timestamps := buildDenseTimestamps(startMs, endMs, step)
	if len(timestamps) == 0 {
		return FEQueryResult{Timestamps: []int64{}, Series: []FESeries{}}
	}

	tsIndex := make(map[int64]int, len(timestamps))
	for i, ts := range timestamps {
		tsIndex[ts] = i
	}

	values := mapPointsToAxis(points, tsIndex, len(timestamps))
	if fillZero {
		zeroFillGaps(values)
	}

	return FEQueryResult{
		Timestamps: timestamps,
		Series:     []FESeries{{Tags: map[string]string{}, Values: values}},
	}
}

type pointGroup struct {
	tags   map[string]string
	points []TimeseriesPoint
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

	timestamps := buildDenseTimestamps(startMs, endMs, step)
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
			zeroFillGaps(values)
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

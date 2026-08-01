package explorer

import (
	"reflect"
	"testing"
	"time"
)

func TestConvertFEQuery(t *testing.T) {
	query := MetricQuery{
		MetricName:  "http.server.duration",
		Aggregation: "p95",
		GroupBy:     []string{"service.name"},
		Where: []Filter{
			{Key: "environment", Operator: "eq", Value: "production"},
			{Key: "http.method", Operator: "not_in", Value: []any{"GET", "HEAD"}},
		},
	}

	f := toFilter(42, 1_000, 61_000, "1m", query)

	if f.TenantID != 42 || f.StartMs != 1_000 || f.EndMs != 61_000 || f.Step != "1m" {
		t.Fatalf("request envelope was not copied: %+v", f)
	}
	if f.MetricName != query.MetricName || f.Aggregation != "p95" {
		t.Fatalf("metric selection was not copied: %+v", f)
	}
	if !reflect.DeepEqual(f.GroupBy, query.GroupBy) {
		t.Fatalf("group by = %#v, want %#v", f.GroupBy, query.GroupBy)
	}
	if got := f.Tags[0]; got.Key != "environment" || got.Operator != "=" || !reflect.DeepEqual(got.Values, []string{"production"}) {
		t.Fatalf("first tag = %#v", got)
	}
	if got := f.Tags[1]; got.Key != "http.method" || got.Operator != "NOT IN" || !reflect.DeepEqual(got.Values, []string{"GET", "HEAD"}) {
		t.Fatalf("second tag = %#v", got)
	}
}

func TestResolveMetricKind(t *testing.T) {
	tests := []struct {
		name       string
		kind       metricNameDTO
		cumulative bool
		histogram  bool
		wantErr    bool
	}{
		{name: "cumulative counter", kind: metricNameDTO{MetricType: "Sum", Temporality: "Cumulative", IsMonotonic: true, Variants: 1}, cumulative: true},
		{name: "delta counter", kind: metricNameDTO{MetricType: "Sum", Temporality: "Delta", IsMonotonic: true, Variants: 1}},
		{name: "non-monotonic sum", kind: metricNameDTO{MetricType: "Sum", Temporality: "Cumulative", Variants: 1}},
		{name: "gauge", kind: metricNameDTO{MetricType: "Gauge", Variants: 1}},
		{name: "histogram", kind: metricNameDTO{MetricType: "Histogram", Temporality: "Delta", Variants: 1}, histogram: true},
		{name: "exponential histogram", kind: metricNameDTO{MetricType: "ExponentialHistogram", Temporality: "Delta", Variants: 1}, histogram: true},
		{name: "summary", kind: metricNameDTO{MetricType: "Summary", Variants: 1}, wantErr: true},
		{name: "unknown", kind: metricNameDTO{MetricType: "Unknown", Variants: 1}, wantErr: true},
		{name: "cumulative histogram", kind: metricNameDTO{MetricType: "Histogram", Temporality: "Cumulative", Variants: 1}, wantErr: true},
		{name: "mixed variants", kind: metricNameDTO{MetricType: "Sum", Variants: 2}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cumulative, histogram, err := resolveMetricKind(tc.kind)
			if (err != nil) != tc.wantErr {
				t.Fatalf("resolveMetricKind() error = %v, wantErr %v", err, tc.wantErr)
			}
			if cumulative != tc.cumulative || histogram != tc.histogram {
				t.Fatalf("resolveMetricKind() = (%v, %v), want (%v, %v)", cumulative, histogram, tc.cumulative, tc.histogram)
			}
		})
	}
}

func TestValidateAggregationForMode(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		aggregation           string
		cumulative, histogram bool
		wantErr               bool
	}{
		{name: "cumulative rate", aggregation: "rate", cumulative: true},
		{name: "cumulative sum", aggregation: "sum", cumulative: true},
		{name: "cumulative average", aggregation: "avg", cumulative: true, wantErr: true},
		{name: "histogram p95", aggregation: "p95", histogram: true},
		{name: "histogram minimum", aggregation: "min", histogram: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAggregationForMode(tc.aggregation, tc.cumulative, tc.histogram)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateAggregationForMode() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestCumulativeValueSupportsSumAndRate(t *testing.T) {
	row := timeseriesPointDTO{Sum: 120}
	if got := cumulativeValue(row, "sum", 60); got != 120 {
		t.Fatalf("cumulative sum = %v, want 120", got)
	}
	if got := cumulativeValue(row, "rate", 60); got != 2 {
		t.Fatalf("cumulative rate = %v, want 2", got)
	}
}

func TestConvertedFilterUsesCommonValidationDefaults(t *testing.T) {
	start := time.Now().UnixMilli()
	f := toFilter(42, start, start+60_000, "1m", MetricQuery{MetricName: "cpu.utilization"})
	if err := f.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if f.Aggregation != "avg" {
		t.Fatalf("aggregation = %q, want avg", f.Aggregation)
	}
}

package explorer

import (
	"testing"
	"time"
)

// For a cumulative counter, row.Sum already carries the per-bucket increase.
// rate must scale it per second; every other aggregation returns the increase.
func TestApplyAggregationCumulative(t *testing.T) {
	rows := []timeseriesPointDTO{
		{BucketAt: time.Unix(0, 0), Sum: 120, Count: 120},
	}

	startMs := int64(0)
	endMs := int64(2 * 60 * 60 * 1000)

	cases := []struct {
		agg  string
		want float64
	}{
		{"sum", 120},
		{"rate", 2},
		{"avg", 120},
		{"count", 120},
		{"max", 120},
	}
	for _, c := range cases {
		got := applyAggregation(rows, c.agg, startMs, endMs, "", true, false)
		if len(got) != 1 || got[0].Value != c.want {
			t.Errorf("cumulative agg %q = %v, want %v", c.agg, got, c.want)
		}
	}
}

func TestApplyAggregationDelta(t *testing.T) {
	rows := []timeseriesPointDTO{
		{BucketAt: time.Unix(0, 0), Sum: 120, Count: 4, Min: 1, Max: 50},
	}
	startMs := int64(0)
	endMs := int64(2 * 60 * 60 * 1000)

	cases := []struct {
		agg  string
		want float64
	}{
		{"sum", 120},
		{"rate", 2},
		{"avg", 30},
		{"min", 1},
		{"max", 50},
		{"count", 4},
	}
	for _, c := range cases {
		got := applyAggregation(rows, c.agg, startMs, endMs, "", false, false)
		if len(got) != 1 || got[0].Value != c.want {
			t.Errorf("delta agg %q = %v, want %v", c.agg, got, c.want)
		}
	}
}

func TestApplyAggregationHistogram(t *testing.T) {
	rows := []timeseriesPointDTO{{
		BucketAt: time.Unix(0, 0),
		Sum:      0, Count: 3,
		HistSum: 500, HistCount: 100,
		Quantiles: []float64{10, 95, 99},
	}}
	startMs := int64(0)
	endMs := int64(2 * 60 * 60 * 1000)

	cases := []struct {
		agg  string
		want float64
	}{
		{"count", 100},
		{"sum", 500},
		{"avg", 5},
		{"rate", 100.0 / 60},
		{"p50", 10},
		{"p95", 95},
		{"p99", 99},
		{"p75", 0},
	}
	for _, c := range cases {
		got := applyAggregation(rows, c.agg, startMs, endMs, "", false, true)
		if len(got) != 1 || got[0].Value != c.want {
			t.Errorf("histogram agg %q = %v, want %v", c.agg, got, c.want)
		}
	}
}

func TestResolveSeriesFlags(t *testing.T) {
	cases := []struct {
		name           string
		kind           *metricKindDTO
		wantCumulative bool
		wantHistogram  bool
	}{
		{
			name:           "nil DTO defaults to false/false",
			kind:           nil,
			wantCumulative: false,
			wantHistogram:  false,
		},
		{
			name:           "Sum + Cumulative + Monotonic = cumulative",
			kind:           &metricKindDTO{Temporality: "Cumulative", IsMonotonic: true, MetricType: "Sum"},
			wantCumulative: true,
			wantHistogram:  false,
		},
		{
			name:           "Sum + Cumulative + non-monotonic = not cumulative",
			kind:           &metricKindDTO{Temporality: "Cumulative", IsMonotonic: false, MetricType: "Sum"},
			wantCumulative: false,
			wantHistogram:  false,
		},
		{
			name:           "Histogram type = histogram flag set",
			kind:           &metricKindDTO{Temporality: "Delta", IsMonotonic: false, MetricType: "Histogram"},
			wantCumulative: false,
			wantHistogram:  true,
		},
		{
			name:           "histogram case insensitive",
			kind:           &metricKindDTO{Temporality: "Delta", IsMonotonic: false, MetricType: "histogram"},
			wantCumulative: false,
			wantHistogram:  true,
		},
		{
			name:           "Gauge = both false",
			kind:           &metricKindDTO{Temporality: "Delta", IsMonotonic: false, MetricType: "Gauge"},
			wantCumulative: false,
			wantHistogram:  false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotCum, gotHist := resolveSeriesFlags(c.kind)
			if gotCum != c.wantCumulative || gotHist != c.wantHistogram {
				t.Errorf("resolveSeriesFlags() = (%v, %v), want (%v, %v)",
					gotCum, gotHist, c.wantCumulative, c.wantHistogram)
			}
		})
	}
}

func TestShouldZeroFill(t *testing.T) {
	cases := []struct {
		name        string
		metricType  string
		aggregation string
		cumulative  bool
		want        bool
	}{
		{"Sum type = zero fill", "Sum", "avg", false, true},
		{"sum lowercase = zero fill", "sum", "avg", false, true},
		{"cumulative flag = zero fill", "Gauge", "avg", true, true},
		{"count aggregation = zero fill", "Gauge", "count", false, true},
		{"rate aggregation = zero fill", "Gauge", "rate", false, true},
		{"gauge + avg = nil fill", "Gauge", "avg", false, false},
		{"summary + avg = nil fill", "Summary", "avg", false, false},
		{"histogram + p50 = nil fill", "Histogram", "p50", false, false},
		{"empty type + avg = nil fill", "", "avg", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shouldZeroFill(c.metricType, c.aggregation, c.cumulative)
			if got != c.want {
				t.Errorf("shouldZeroFill(%q, %q, %v) = %v, want %v",
					c.metricType, c.aggregation, c.cumulative, got, c.want)
			}
		})
	}
}

// TestBuildColumnarResult_FullBuckets verifies that when every bucket has data,
// all values are real pointers and the dense axis matches expectations.
func TestBuildColumnarResult_FullBuckets(t *testing.T) {
	// 1-hour window with 15m step → 5 buckets: 0, 900000, 1800000, 2700000, 3600000
	// (endMs=3600000 is inclusive in the dense axis loop)
	startMs := int64(0)
	endMs := int64(3600000)
	step := "15m"

	points := []TimeseriesPoint{
		{Timestamp: "1970-01-01 00:00:00", Value: 1.0},
		{Timestamp: "1970-01-01 00:15:00", Value: 2.0},
		{Timestamp: "1970-01-01 00:30:00", Value: 3.0},
		{Timestamp: "1970-01-01 00:45:00", Value: 4.0},
		{Timestamp: "1970-01-01 01:00:00", Value: 5.0},
	}

	result := buildColumnarResult(points, startMs, endMs, step, true)

	wantTs := []int64{0, 900000, 1800000, 2700000, 3600000}
	if len(result.Timestamps) != len(wantTs) {
		t.Fatalf("timestamps length = %d, want %d", len(result.Timestamps), len(wantTs))
	}
	for i, ts := range result.Timestamps {
		if ts != wantTs[i] {
			t.Errorf("timestamps[%d] = %d, want %d", i, ts, wantTs[i])
		}
	}

	if len(result.Series) != 1 {
		t.Fatalf("series count = %d, want 1", len(result.Series))
	}
	wantVals := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	for i, v := range result.Series[0].Values {
		if v == nil {
			t.Errorf("values[%d] = nil, want %v", i, wantVals[i])
		} else if *v != wantVals[i] {
			t.Errorf("values[%d] = %v, want %v", i, *v, wantVals[i])
		}
	}
}

// TestBuildColumnarResult_ZeroFill verifies that sparse points with fillZero=true
// produce 0.0 in missing buckets while preserving real values.
func TestBuildColumnarResult_ZeroFill(t *testing.T) {
	// 1-hour window with 15m step → 5 buckets
	// Supply data only for buckets 0 and 2
	startMs := int64(0)
	endMs := int64(3600000)
	step := "15m"

	points := []TimeseriesPoint{
		{Timestamp: "1970-01-01 00:00:00", Value: 10.0},
		{Timestamp: "1970-01-01 00:30:00", Value: 30.0},
	}

	result := buildColumnarResult(points, startMs, endMs, step, true)

	wantTs := []int64{0, 900000, 1800000, 2700000, 3600000}
	if len(result.Timestamps) != len(wantTs) {
		t.Fatalf("timestamps length = %d, want %d", len(result.Timestamps), len(wantTs))
	}

	if len(result.Series) != 1 {
		t.Fatalf("series count = %d, want 1", len(result.Series))
	}

	vals := result.Series[0].Values
	// Bucket 0: real value 10.0
	if vals[0] == nil || *vals[0] != 10.0 {
		t.Errorf("values[0] = %v, want 10.0", vals[0])
	}
	// Bucket 1: zero-filled
	if vals[1] == nil || *vals[1] != 0.0 {
		t.Errorf("values[1] = %v, want 0.0 (zero-filled)", vals[1])
	}
	// Bucket 2: real value 30.0
	if vals[2] == nil || *vals[2] != 30.0 {
		t.Errorf("values[2] = %v, want 30.0", vals[2])
	}
	// Bucket 3: zero-filled
	if vals[3] == nil || *vals[3] != 0.0 {
		t.Errorf("values[3] = %v, want 0.0 (zero-filled)", vals[3])
	}
	// Bucket 4: zero-filled
	if vals[4] == nil || *vals[4] != 0.0 {
		t.Errorf("values[4] = %v, want 0.0 (zero-filled)", vals[4])
	}
}

// TestBuildColumnarResult_NilFill verifies that sparse points with fillZero=false
// produce nil in missing buckets (gauge behavior).
func TestBuildColumnarResult_NilFill(t *testing.T) {
	startMs := int64(0)
	endMs := int64(3600000)
	step := "15m"

	points := []TimeseriesPoint{
		{Timestamp: "1970-01-01 00:00:00", Value: 10.0},
		{Timestamp: "1970-01-01 00:30:00", Value: 30.0},
	}

	result := buildColumnarResult(points, startMs, endMs, step, false)

	if len(result.Series) != 1 {
		t.Fatalf("series count = %d, want 1", len(result.Series))
	}

	vals := result.Series[0].Values
	// Bucket 0: real value 10.0
	if vals[0] == nil || *vals[0] != 10.0 {
		t.Errorf("values[0] = %v, want 10.0", vals[0])
	}
	// Bucket 1: nil (gauge gap)
	if vals[1] != nil {
		t.Errorf("values[1] = %v, want nil (gauge gap)", *vals[1])
	}
	// Bucket 2: real value 30.0
	if vals[2] == nil || *vals[2] != 30.0 {
		t.Errorf("values[2] = %v, want 30.0", vals[2])
	}
	// Bucket 3: nil (gauge gap)
	if vals[3] != nil {
		t.Errorf("values[3] = %v, want nil (gauge gap)", *vals[3])
	}
}

// TestBuildColumnarResult_Empty verifies that no points returns empty arrays.
func TestBuildColumnarResult_Empty(t *testing.T) {
	startMs := int64(0)
	endMs := int64(3600000)
	step := "15m"

	result := buildColumnarResult(nil, startMs, endMs, step, true)

	// Dense axis should still be generated even with no points.
	wantTs := []int64{0, 900000, 1800000, 2700000, 3600000}
	if len(result.Timestamps) != len(wantTs) {
		t.Fatalf("timestamps length = %d, want %d", len(result.Timestamps), len(wantTs))
	}
	if len(result.Series) != 1 {
		t.Fatalf("series count = %d, want 1", len(result.Series))
	}
	// All values should be zero-filled.
	for i, v := range result.Series[0].Values {
		if v == nil || *v != 0.0 {
			t.Errorf("values[%d] = %v, want 0.0 (zero-filled empty)", i, v)
		}
	}
}

func TestBuildGroupedColumnarResult(t *testing.T) {
	startMs := int64(0)
	endMs := int64(900000)
	step := "15m"
	rows := []timeseriesPointDTO{
		{BucketAt: time.Unix(0, 0), GroupValues: []string{"api", "prod"}, Sum: 10, Count: 1},
		{BucketAt: time.Unix(0, 0), GroupValues: []string{"worker", "prod"}, Sum: 20, Count: 1},
		{BucketAt: time.Unix(900, 0), GroupValues: []string{"api", "prod"}, Sum: 30, Count: 1},
	}
	points := applyAggregation(rows, "sum", startMs, endMs, step, false, false)

	result := buildGroupedColumnarResult(
		rows, points, []string{"service", "environment"}, startMs, endMs, step, false,
	)

	if len(result.Series) != 2 {
		t.Fatalf("series count = %d, want 2", len(result.Series))
	}
	api := result.Series[0]
	if api.Tags["service"] != "api" || api.Tags["environment"] != "prod" {
		t.Fatalf("api tags = %v", api.Tags)
	}
	if api.Values[0] == nil || *api.Values[0] != 10 || api.Values[1] == nil || *api.Values[1] != 30 {
		t.Fatalf("api values = %v", api.Values)
	}
	worker := result.Series[1]
	if worker.Tags["service"] != "worker" || worker.Values[0] == nil || *worker.Values[0] != 20 {
		t.Fatalf("worker series = %+v", worker)
	}
	if worker.Values[1] != nil {
		t.Fatalf("worker missing bucket = %v, want nil", *worker.Values[1])
	}
}

func TestBuildGroupedColumnarResultEmpty(t *testing.T) {
	result := buildGroupedColumnarResult(nil, nil, []string{"service"}, 0, 900000, "15m", true)
	if len(result.Timestamps) != 2 || len(result.Series) != 0 {
		t.Fatalf("result = %+v, want dense timestamps and no grouped series", result)
	}
}

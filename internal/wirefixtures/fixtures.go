package main

import (
	"bytes"
	"encoding/json"
	"time"

	logsmodels "github.com/optikklabs/query/internal/modules/logs/shared/models"
	metricsexplorer "github.com/optikklabs/query/internal/modules/metrics/explorer"
	dbquerydetail "github.com/optikklabs/query/internal/modules/saturation/database/querydetail"
	dbslowqueries "github.com/optikklabs/query/internal/modules/saturation/database/slowqueries"
	kafkatopology "github.com/optikklabs/query/internal/modules/saturation/kafka/topology"
	redfleet "github.com/optikklabs/query/internal/modules/services/redfleet"
	topology "github.com/optikklabs/query/internal/modules/services/topology"
	tracesdetail "github.com/optikklabs/query/internal/modules/traces/detail"
	tracesexplorer "github.com/optikklabs/query/internal/modules/traces/explorer"
	tracespaths "github.com/optikklabs/query/internal/modules/traces/paths"
	tracesservicemap "github.com/optikklabs/query/internal/modules/traces/servicemap"
)

// fixtureTime is fixed so the encoding is byte-stable across runs; a golden
// file that churns on every invocation would be worthless as a diff.
var fixtureTime = time.Date(2026, 7, 16, 10, 30, 0, 0, time.UTC)

// buildFixtures returns one sample of every response struct the web client
// parses. Each entry pairs a fully-populated value with a zero value: the zero
// value is what proves which keys survive `omitempty` and which nil maps and
// slices encode as null.
func buildFixtures() map[string]any {
	now := fixtureTime

	return map[string]any{
		"traceSpansEnvelope": map[string]any{
			"spans": []tracesdetail.SpanListItem{{
				SpanID: "a1", ParentSpanID: "", TraceID: "t1",
				ServiceName: "gateway", OperationName: "GET /x",
				KindString: "SERVER", StatusCode: "OK",
				HasError: false, DurationMs: 12.5, StartNs: now.UnixNano(),
			}, {}},
		},
		"spanEvents": []tracesdetail.SpanEvent{{SpanID: "a1", TraceID: "t1", EventName: "e", Timestamp: now, Attributes: "{}"}, {}},
		// GetSpanAttributes always initialises both maps, so they are never
		// null on the wire even when the span carries no attributes.
		"spanAttributes": tracesdetail.SpanAttributes{
			SpanID: "a1", TraceID: "t1", OperationName: "op", ServiceName: "svc",
			AttributesString: map[string]string{},
			ResourceAttrs:    map[string]string{},
		},
		"spanAttributesFull": tracesdetail.SpanAttributes{
			SpanID: "a1", TraceID: "t1", OperationName: "op", ServiceName: "svc",
			AttributesString: map[string]string{"k": "v"},
			ResourceAttrs:    map[string]string{"r": "v"},
			ExceptionType:    "E", DBSystem: "mysql",
			Attributes: map[string]string{"a": "b"},
			Links:      []tracesdetail.SpanLink{{TraceID: "t2", SpanID: "b1"}},
		},
		"relatedTraces": []tracesdetail.RelatedTrace{{TraceID: "t2", SpanID: "b1", OperationName: "op", ServiceName: "s", DurationMs: 1, Status: "OK", StartTime: now}, {}},
		"criticalPath":  []tracespaths.CriticalPathSpan{{SpanID: "a1", OperationName: "op", ServiceName: "s", DurationMs: 3}, {}},
		"errorPath":     []tracespaths.ErrorPathSpan{{SpanID: "a1", ParentSpanID: "", OperationName: "op", ServiceName: "s", Status: "ERROR", StatusMessage: "m", StartTime: now, DurationMs: 3}, {}},
		"traceErrors":   []tracesservicemap.TraceErrorGroup{{ExceptionType: "E", Count: 1, Spans: []tracesservicemap.TraceErrorSpan{{SpanID: "a1", ServiceName: "s", OperationName: "op", StartTime: now, DurationMs: 2}, {}}}},
		"tracesQuery":   tracesexplorer.QueryResponse{Results: []tracesexplorer.Trace{{TraceID: "t1", StartMs: 1, EndMs: 2, DurationMs: 1, RootService: "s", RootOperation: "op", SpanCount: 3, HasError: true, ErrorCount: 1}, {}}, PageInfo: tracesexplorer.PageInfo{HasMore: true, NextCursor: "c", Limit: 50}},
		"tracesFacets":  tracesexplorer.Facets{Service: []tracesexplorer.FacetBucket{{Value: "s", Count: 2}}},
		"tracesTrend":   []tracesexplorer.TrendBucket{{TimeBucket: "2026-07-16T10:00:00Z", Total: 5, Errors: 1}, {}},
		"suggest":       tracesexplorer.SuggestResponse{Suggestions: []tracesexplorer.Suggestion{{Value: "v", Count: 1}}},
		"logsQuery": struct {
			Results  []logsmodels.Log    `json:"results"`
			PageInfo logsmodels.PageInfo `json:"pageInfo"`
		}{Results: []logsmodels.Log{{ID: "l1", Timestamp: uint64(now.UnixNano()), ObservedTimestamp: uint64(now.UnixNano()), SeverityText: "INFO", SeverityNumber: 9, SeverityBucket: 2, Body: "hi", ServiceName: "s"}, {}}, PageInfo: logsmodels.PageInfo{HasMore: false, Limit: 100}},
		"traceLogs": []logsmodels.Log{{ID: "l1", Timestamp: uint64(now.UnixNano()), ObservedTimestamp: uint64(now.UnixNano()), SeverityText: "INFO", Body: "hi", ServiceName: "s", AttributesString: map[string]string{"k": "v"}}, {}},
		"logsSummary": struct {
			Summary logsmodels.Summary `json:"summary"`
		}{Summary: logsmodels.Summary{Total: 10, Errors: 2, Warns: 1}},
		"logsTrend": struct {
			Trend []logsmodels.TrendBucket `json:"trend"`
		}{Trend: []logsmodels.TrendBucket{{TimeBucket: "t", Total: 1}, {}}},
		"logsFacets": struct {
			Facets logsmodels.Facets `json:"facets"`
		}{Facets: logsmodels.Facets{Severity: []string{"INFO"}, Service: []logsmodels.FacetValue{{Value: "s", Count: 1}}}},
		"metricNames":  metricsexplorer.FEMetricNamesResponse{Metrics: []metricsexplorer.FEMetricNameEntry{{Name: "m", Type: "gauge"}, {Name: "m2", Type: "counter", Unit: "s", Description: "d"}}},
		"metricTags":   metricsexplorer.FETagsResponse{Tags: []metricsexplorer.FETagEntry{{Key: "k", Values: []string{"v"}}}},
		"metricsQuery": metricsexplorer.FEQueryResponse{Results: map[string]metricsexplorer.FEQueryResult{"a": {Timestamps: []int64{1}, Series: []metricsexplorer.FESeries{{Tags: map[string]string{"k": "v"}, Values: []*float64{nil}}}}}},
		"topology":     topology.BuildGraph(nil, nil),
		"redServices":  []redfleet.ServiceREDMetric{{ServiceName: "s", RequestCount: 1, ErrorCount: 0, AvgLatency: 1, P95Latency: 2, P99Latency: 3}, {}},
		"kafkaTopology": kafkatopology.TopologyResponse{
			Producers: []kafkatopology.ProducerNode{{Service: "p"}},
			Topics:    []kafkatopology.TopicNode{{Topic: "t"}},
			Consumers: []kafkatopology.ConsumerNode{{Service: "c", Group: "g"}},
			Edges:     []kafkatopology.StreamEdge{{Source: "p", Target: "t", Kind: "produce"}},
			Pathways:  []kafkatopology.Pathway{{Producer: "p", Topic: "t", Group: "g", Consumer: "c"}},
		},
		"querySummary":    dbquerydetail.QuerySummary{QueryHash: "h", Services: []dbquerydetail.ServiceCalls{{Service: "s", CallCount: 1}}},
		"queryTimeseries": []dbquerydetail.QueryTimeseriesPoint{{TimeBucket: "t", CallCount: 1}, {}},
		"queryExecutions": []dbquerydetail.QueryExecution{{Timestamp: "t", TraceID: "t1", SpanID: "s1", DurationMs: 1, IsError: false, Service: "s", Host: "h"}, {}},
		"slowQueries":     []dbslowqueries.SlowQueryPattern{{QueryHash: "h", QueryText: "SELECT 1", DBSystem: "postgresql", CollectionName: "c", Namespace: "app", Server: "db.internal", CallCount: 1}, {}},
	}
}

// encodeFixtures renders the fixtures as indented JSON. Map keys are sorted by
// encoding/json, so the output is deterministic and safe to diff.
func encodeFixtures() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(buildFixtures()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

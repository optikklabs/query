package service

import (
	"sort"

	"github.com/optikklabs/query/internal/modules/services/topology"
	"github.com/optikklabs/query/internal/modules/traces/models"
	"github.com/optikklabs/query/internal/modules/traces/repository"
)

// buildServiceMap derives per-trace service topology from the span set:
// nodes per service, edges per parent->child service hop.
func buildServiceMap(rows []repository.TraceSpanRow) topology.TopologyResponse {
	return topology.BuildGraph(nodeAggsFromSpans(rows), edgeAggsFromSpans(rows))
}

func nodeAggsFromSpans(rows []repository.TraceSpanRow) []topology.NodeAgg {
	aggMap := make(map[string]*topology.NodeAgg)
	for i := range rows {
		r := &rows[i]
		if r.ServiceName == "" {
			continue
		}
		a, ok := aggMap[r.ServiceName]
		if !ok {
			a = &topology.NodeAgg{Service: r.ServiceName}
			aggMap[r.ServiceName] = a
		}
		a.RequestCount++
		a.P50Ms += r.DurationMs()
		if r.HasError {
			a.ErrorCount++
		}
	}
	out := make([]topology.NodeAgg, 0, len(aggMap))
	for _, a := range aggMap {
		if a.RequestCount > 0 {
			a.P50Ms /= float64(a.RequestCount)
		}
		out = append(out, *a)
	}
	return out
}

func edgeAggsFromSpans(rows []repository.TraceSpanRow) []topology.EdgeAgg {
	bySpan := make(map[string]*repository.TraceSpanRow, len(rows))
	for i := range rows {
		bySpan[rows[i].SpanID] = &rows[i]
	}
	aggMap := make(map[[2]string]*topology.EdgeAgg)
	for i := range rows {
		child := &rows[i]
		parent := bySpan[child.ParentSpanID]
		if parent == nil || parent.ServiceName == "" || child.ServiceName == "" || parent.ServiceName == child.ServiceName {
			continue
		}
		key := [2]string{parent.ServiceName, child.ServiceName}
		a, ok := aggMap[key]
		if !ok {
			a = &topology.EdgeAgg{Source: parent.ServiceName, Target: child.ServiceName}
			aggMap[key] = a
		}
		a.CallCount++
		a.P50Ms += child.DurationMs()
		if child.HasError {
			a.ErrorCount++
		}
	}
	out := make([]topology.EdgeAgg, 0, len(aggMap))
	for _, a := range aggMap {
		if a.CallCount > 0 {
			a.P50Ms /= float64(a.CallCount)
		}
		out = append(out, *a)
	}
	return out
}

// groupErrors buckets error spans by exception type (falling back to
// status message), most frequent group first.
func groupErrors(rows []repository.TraceSpanRow) []models.TraceErrorGroup {
	groups := make(map[string]*models.TraceErrorGroup)
	for i := range rows {
		r := &rows[i]
		key := errorGroupKey(r)
		g, ok := groups[key]
		if !ok {
			g = &models.TraceErrorGroup{ExceptionType: key}
			groups[key] = g
		}
		g.Count++
		g.Spans = append(g.Spans, models.TraceErrorSpan{
			SpanID:           r.SpanID,
			ServiceName:      r.ServiceName,
			OperationName:    r.OperationName,
			ExceptionMessage: r.ExceptionMessage,
			StatusMessage:    r.StatusMessage,
			StartTime:        r.Timestamp,
			DurationMs:       r.DurationMs(),
		})
	}
	out := make([]models.TraceErrorGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}

func errorGroupKey(r *repository.TraceSpanRow) string {
	if r.ExceptionType != "" {
		return r.ExceptionType
	}
	if r.StatusMessage != "" {
		return r.StatusMessage
	}
	return "UnknownError"
}

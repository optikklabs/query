package service

import (
	"context"
	"log/slog"
	"sort"

	"github.com/optikklabs/query/internal/modules/services/topology"
	"github.com/optikklabs/query/internal/modules/traces/models"
	"github.com/optikklabs/query/internal/modules/traces/repository"
)

func (s *Service) GetServiceMap(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) (topology.TopologyResponse, error) {
	rows, err := s.repo.GetServiceMapSpans(ctx, tenantID, traceID, startMs, endMs)
	if err != nil {
		slog.ErrorContext(ctx, "servicemap: GetServiceMap failed", slog.Any("error", err), slog.Int64("tenant_id", tenantID), slog.String("trace_id", traceID))
		return topology.TopologyResponse{}, err
	}
	return topology.BuildGraph(nodeAggsFromSpans(rows), edgeAggsFromSpans(rows)), nil
}

func (s *Service) GetTraceErrors(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) ([]models.TraceErrorGroup, error) {
	rows, err := s.repo.GetTraceErrors(ctx, tenantID, traceID, startMs, endMs)
	if err != nil {
		slog.ErrorContext(ctx, "servicemap: GetTraceErrors failed", slog.Any("error", err), slog.Int64("tenant_id", tenantID), slog.String("trace_id", traceID))
		return nil, err
	}
	return groupErrors(rows), nil
}

func nodeAggsFromSpans(rows []repository.ServiceMapSpanRow) []topology.NodeAgg {
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
		a.P50Ms += r.DurationMs
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

func edgeAggsFromSpans(rows []repository.ServiceMapSpanRow) []topology.EdgeAgg {
	bySpan := make(map[string]*repository.ServiceMapSpanRow, len(rows))
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
		a.P50Ms += child.DurationMs
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

func groupErrors(rows []repository.TraceErrorRow) []models.TraceErrorGroup {
	groups := make(map[string]*models.TraceErrorGroup)
	for _, r := range rows {
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
			StartTime:        r.StartTime,
			DurationMs:       r.DurationMs,
		})
	}
	out := make([]models.TraceErrorGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}

func errorGroupKey(r repository.TraceErrorRow) string {
	if r.ExceptionType != "" {
		return r.ExceptionType
	}
	if r.StatusMessage != "" {
		return r.StatusMessage
	}
	return "UnknownError"
}

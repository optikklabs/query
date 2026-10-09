package service

import (
	"context"

	"github.com/optikklabs/query/internal/modules/infrastructure/models"
	"github.com/optikklabs/query/internal/shared/metrics"
	"github.com/optikklabs/query/internal/shared/nullable"
)

func (s *Service) GetInfrastructureNodes(ctx context.Context, tenantID int64, startMs, endMs int64) ([]models.InfrastructureNode, error) {
	rows, err := s.repo.QueryInfrastructureNodes(ctx, tenantID, startMs, endMs)
	if err != nil {
		return nil, err
	}
	out := make([]models.InfrastructureNode, len(rows))
	for i, r := range rows {
		errorRate, avgLatency := metrics.REDDerivations(r.RequestCount, r.ErrorCount, r.DurationMsSum)
		out[i] = models.InfrastructureNode{
			Host:         r.Host,
			PodCount:     int64(r.PodCount),
			Services:     nullable.OrEmpty(r.Services),
			RequestCount: int64(r.RequestCount),
			ErrorCount:   int64(r.ErrorCount),
			ErrorRate:    errorRate,
			AvgLatencyMs: avgLatency,
			P95LatencyMs: r.P95LatencyMs,
			LastSeen:     r.LastSeen,
		}
	}
	return out, nil
}

func (s *Service) GetInfrastructureNodeSummary(ctx context.Context, tenantID int64, startMs, endMs int64) (models.InfrastructureNodeSummary, error) {
	row, err := s.repo.QueryInfrastructureNodeSummary(ctx, tenantID, startMs, endMs)
	if err != nil {
		return models.InfrastructureNodeSummary{}, err
	}
	return models.InfrastructureNodeSummary{
		HealthyNodes:   int64(row.HealthyNodes),
		DegradedNodes:  int64(row.DegradedNodes),
		UnhealthyNodes: int64(row.UnhealthyNodes),
		TotalPods:      int64(row.TotalPods),
	}, nil
}

func (s *Service) GetInfrastructureNodeServices(ctx context.Context, tenantID int64, host string, startMs, endMs int64) ([]models.InfrastructureNodeService, error) {
	rows, err := s.repo.QueryInfrastructureNodeServices(ctx, tenantID, host, startMs, endMs)
	if err != nil {
		return nil, err
	}
	out := make([]models.InfrastructureNodeService, len(rows))
	for i, r := range rows {
		errorRate, avgLatency := metrics.REDDerivations(r.RequestCount, r.ErrorCount, r.DurationMsSum)
		out[i] = models.InfrastructureNodeService{
			ServiceName:  r.Service,
			RequestCount: int64(r.RequestCount),
			ErrorCount:   int64(r.ErrorCount),
			ErrorRate:    errorRate,
			AvgLatencyMs: avgLatency,
			P95LatencyMs: r.P95LatencyMs,
			PodCount:     int64(r.PodCount),
		}
	}
	return out, nil
}

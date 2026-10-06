package service

import (
	"context"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/models"
)

func (s *Service) GetAvgCPU(ctx context.Context, tenantID int64, startMs, endMs int64) (models.MetricValue, error) {
	byMetric, err := s.repo.QueryMetricAverages(ctx, tenantID, startMs, endMs, infraconsts.CPUMetrics)
	if err != nil {
		return models.MetricValue{}, err
	}
	return models.MetricValue{Value: valueOrZero(foldCPU(byMetric))}, nil
}

func (s *Service) GetAvgMemory(ctx context.Context, tenantID int64, startMs, endMs int64) (models.MetricValue, error) {
	byMetric, err := s.repo.QueryMetricAverages(ctx, tenantID, startMs, endMs, infraconsts.MemoryMetrics)
	if err != nil {
		return models.MetricValue{}, err
	}
	return models.MetricValue{Value: valueOrZero(foldMem(byMetric))}, nil
}

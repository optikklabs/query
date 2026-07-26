package service

import (
	"context"
	"math"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/models"
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
)

func (s *Service) GetAvgMemory(ctx context.Context, tenantID int64, startMs, endMs int64) (models.MetricValue, error) {
	rows, err := s.repo.QueryMemoryUtilizationAgg(ctx, tenantID, startMs, endMs)
	if err != nil {
		return models.MetricValue{}, err
	}
	avg := foldMemoryMetricRows(rows)
	if avg == nil {
		return models.MetricValue{Value: 0}, nil
	}
	return models.MetricValue{Value: *avg}, nil
}

func (s *Service) GetMemoryByInstance(ctx context.Context, tenantID int64, host, pod, container, serviceName string, startMs, endMs int64) (*float64, error) {
	_ = container
	rows, err := s.repo.QueryMemoryUtilizationForInstance(ctx, tenantID, startMs, endMs, host, pod, serviceName)
	if err != nil {
		return nil, err
	}
	return foldMemoryMetricRows(rows), nil
}

// foldMemoryMetricRows blends memory metrics into a utilization percentage.
func foldMemoryMetricRows(rows []repository.MemoryMetricNameRow) *float64 {
	by := make(map[string]float64, len(rows))
	for _, r := range rows {
		by[r.MetricName] = r.Value
	}
	var values []float64
	if v, ok := by[infraconsts.MetricSystemMemoryUtilization]; ok {
		if !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 {
			if v <= infraconsts.PercentageThreshold {
				v = v * infraconsts.PercentageMultiplier
			}
			values = append(values, v)
		}
	}
	if max := by[infraconsts.MetricJVMMemoryMax]; max > 0 {
		used := by[infraconsts.MetricJVMMemoryUsed]
		values = append(values, infraconsts.PercentageMultiplier*used/max)
	}
	return averageFloats(values)
}

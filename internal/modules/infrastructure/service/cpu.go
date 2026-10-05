package service

import (
	"context"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/models"
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
)

func (s *Service) GetAvgCPU(ctx context.Context, tenantID int64, startMs, endMs int64) (models.MetricValue, error) {
	rows, err := s.repo.QueryCPUUtilizationAgg(ctx, tenantID, startMs, endMs)
	if err != nil {
		return models.MetricValue{}, err
	}
	avg := foldCPUMetricRows(rows)
	if avg == nil {
		return models.MetricValue{Value: 0}, nil
	}
	return models.MetricValue{Value: *avg}, nil
}

func foldCPUMetricRows(rows []repository.CPUMetricNameRow) *float64 {
	byMetric := map[string]float64{}
	for _, r := range rows {
		byMetric[r.MetricName] = r.Value
	}
	var values []float64
	add := func(v float64) {
		if nv := infraconsts.NormalizeUtilization(v); nv != nil {
			values = append(values, *nv)
		}
	}
	if v, ok := byMetric[infraconsts.MetricSystemCPUUtilization]; ok {
		add(v)
	}
	if v, ok := byMetric[infraconsts.MetricSystemCPUUsage]; ok {
		add(v)
	}
	if v, ok := byMetric[infraconsts.MetricProcessCPUUsage]; ok {
		add(v)
	}
	return infraconsts.AverageUtilization(values)
}

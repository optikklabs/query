package service

import (
	"context"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/models"
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
)

// GetAvgCPU folds the 3-metric utilization family across the window.
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

func (s *Service) GetCPUByInstance(ctx context.Context, tenantID int64, startMs, endMs int64) ([]models.CPUInstanceMetric, error) {
	rows, err := s.repo.QueryCPUUtilizationByInstance(ctx, tenantID, startMs, endMs)
	if err != nil {
		return nil, err
	}
	type instKey struct{ host, pod, container, service string }
	byInst := map[instKey][]repository.CPUMetricNameRow{}
	order := []instKey{}
	for _, r := range rows {
		k := instKey{r.Host, r.Pod, r.Container, r.Service}
		if _, ok := byInst[k]; !ok {
			order = append(order, k)
		}
		byInst[k] = append(byInst[k], repository.CPUMetricNameRow{MetricName: r.MetricName, Value: r.Value})
	}
	out := make([]models.CPUInstanceMetric, 0, len(order))
	for _, k := range order {
		out = append(out, models.CPUInstanceMetric{
			Host:        k.host,
			Pod:         k.pod,
			Container:   k.container,
			ServiceName: k.service,
			Value:       foldCPUMetricRows(byInst[k]),
		})
	}
	return out, nil
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
	return averageFloats(values)
}

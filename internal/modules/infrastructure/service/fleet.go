package service

import (
	"context"

	"github.com/optikklabs/query/internal/modules/infrastructure/models"
	"github.com/optikklabs/query/internal/shared/metrics"
	"github.com/optikklabs/query/internal/shared/nullable"
)

func (s *Service) GetFleetPods(ctx context.Context, tenantID int64, startMs, endMs int64, host string) ([]models.FleetPod, error) {
	rows, err := s.repo.QueryFleetPods(ctx, tenantID, startMs, endMs, host)
	if err != nil {
		return nil, err
	}
	out := make([]models.FleetPod, len(rows))
	for i, r := range rows {
		errorRate, avgLatency := metrics.REDDerivations(r.RequestCount, r.ErrorCount, r.DurationMsSum)
		out[i] = models.FleetPod{
			PodName:      r.Pod,
			Host:         r.Host,
			Services:     nullable.OrEmpty(r.Services),
			RequestCount: int64(r.RequestCount),
			ErrorCount:   int64(r.ErrorCount),
			ErrorRate:    errorRate,
			AvgLatencyMs: avgLatency,
			P95LatencyMs: float64(r.P95LatencyMs),
			LastSeen:     r.LastSeen,
		}
	}
	return out, nil
}

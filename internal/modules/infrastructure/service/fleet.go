package service

import (
	"context"
	"time"

	"github.com/optikklabs/query/internal/modules/infrastructure/models"
	"github.com/optikklabs/query/internal/shared/metrics"
)

func (s *Service) GetFleetPods(ctx context.Context, tenantID int64, startMs, endMs int64, host string) ([]models.FleetPod, error) {
	rows, err := s.repo.QueryFleetPods(ctx, tenantID, startMs, endMs, host)
	if err != nil {
		return nil, err
	}
	out := make([]models.FleetPod, len(rows))
	for i, r := range rows {
		errorRate, avgLatency := metrics.REDDerivations(r.RequestCount, r.ErrorCount, r.DurationMsSum)
		services := r.Services
		if services == nil {
			services = []string{}
		}
		out[i] = models.FleetPod{
			PodName:      r.Pod,
			Host:         r.Host,
			Services:     services,
			RequestCount: int64(r.RequestCount),
			ErrorCount:   int64(r.ErrorCount),
			ErrorRate:    errorRate,
			AvgLatencyMs: avgLatency,
			P95LatencyMs: float64(r.P95LatencyMs),
			LastSeen:     r.LastSeen.Format(time.RFC3339),
		}
	}
	return out, nil
}

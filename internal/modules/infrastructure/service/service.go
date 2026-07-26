// Package service holds the infrastructure domain's business logic: it folds
// repository row types into the API models, normalizes utilization metrics to
// percentages, and derives RED rates.
package service

import (
	"math"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
	"github.com/optikklabs/query/internal/shared/metrics"
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service { return &Service{repo: repo} }

// normalizeUtilization coerces a utilization reading to a 0..100 percentage,
// rejecting values that cannot be one. Agents report either a 0..1 fraction or
// an already-scaled percentage, and the two are told apart by magnitude.
func normalizeUtilization(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > infraconsts.PercentageThreshold*100 {
		return nil
	}
	if v <= infraconsts.PercentageThreshold {
		v *= infraconsts.PercentageMultiplier
	}
	return &v
}

// averageFloats means the valid entries, skipping anything non-finite or
// negative. Nil means nothing usable was present — distinct from an average
// of zero.
func averageFloats(values []float64) *float64 {
	var sum float64
	count := 0
	for _, v := range values {
		if !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 {
			sum += v
			count++
		}
	}
	if count == 0 {
		return nil
	}
	avg := sum / float64(count)
	return &avg
}

// redDerivations computes error percentage and mean latency, guarding the
// zero-request case that would otherwise divide by zero.
func redDerivations(reqCount, errCount uint64, durationMsSum float64) (errorRate, avgLatency float64) {
	if reqCount == 0 {
		return 0, 0
	}
	return metrics.Percentage(errCount, reqCount), durationMsSum / float64(reqCount)
}

func ptr(v float64) *float64 { return &v }

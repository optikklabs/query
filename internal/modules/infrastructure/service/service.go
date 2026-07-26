// Package service holds the infrastructure domain's business logic: it folds
// repository row types into the API models, normalizes utilization metrics to
// percentages, and derives RED rates.
package service

import (
	"math"

	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service { return &Service{repo: repo} }

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

func ptr(v float64) *float64 { return &v }

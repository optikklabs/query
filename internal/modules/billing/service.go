package billing

import (
	"context"
	"time"
)

// Service owns trial-lifecycle business logic.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// SweepExpiredTrials suspends every trial whose deadline has passed. Returns the
// number of tenants suspended on this tick.
func (s *Service) SweepExpiredTrials(ctx context.Context, now time.Time) (int64, error) {
	return s.repo.SuspendExpiredTrials(ctx, now.UTC())
}

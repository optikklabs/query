package billing

import (
	"context"
	"time"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SweepExpiredTrials(ctx context.Context, now time.Time) (int64, error) {
	return s.repo.SuspendExpiredTrials(ctx, now.UTC())
}

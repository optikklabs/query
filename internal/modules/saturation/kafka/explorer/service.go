package explorer

import (
	"context"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Topic Domains
func (s *Service) GetTopicThroughput(ctx context.Context, tenantID, startMs, endMs int64, topic string) ([]TopicThroughputRow, error) {
	return s.repo.QueryTopicThroughput(ctx, tenantID, startMs, endMs, topic)
}
func (s *Service) GetGroupPartitions(ctx context.Context, tenantID, startMs, endMs int64, group string) ([]GroupPartitionsRow, error) {
	return s.repo.QueryGroupPartitions(ctx, tenantID, startMs, endMs, group)
}

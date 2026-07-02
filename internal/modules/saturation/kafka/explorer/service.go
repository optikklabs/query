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
func (s *Service) GetTopicThroughput(ctx context.Context, teamID, startMs, endMs int64, topic string) ([]TopicThroughputRow, error) {
	return s.repo.QueryTopicThroughput(ctx, teamID, startMs, endMs, topic)
}
func (s *Service) GetGroupPartitions(ctx context.Context, teamID, startMs, endMs int64, group string) ([]GroupPartitionsRow, error) {
	return s.repo.QueryGroupPartitions(ctx, teamID, startMs, endMs, group)
}

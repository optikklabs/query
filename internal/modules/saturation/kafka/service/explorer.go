package service

import (
	"context"

	"github.com/optikklabs/query/internal/modules/saturation/kafka/models"
)

func (s *Service) GetTopicThroughput(ctx context.Context, tenantID, startMs, endMs int64, topic string) ([]models.TopicThroughputRow, error) {
	return s.repo.QueryTopicThroughput(ctx, tenantID, startMs, endMs, topic)
}

func (s *Service) GetGroupPartitions(ctx context.Context, tenantID, startMs, endMs int64, group string) ([]models.GroupPartitionsRow, error) {
	return s.repo.QueryGroupPartitions(ctx, tenantID, startMs, endMs, group)
}

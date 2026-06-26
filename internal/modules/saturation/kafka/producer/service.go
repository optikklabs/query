package producer

import (
	"context"
	"time"

	"github.com/optikklabs/query/internal/modules/saturation/kafka/filter"
)

// publishRateRepo is the repository surface the service depends on (DIP: lets
// tests inject synthetic counter rows without a ClickHouse connection).
type publishRateRepo interface {
	QueryPublishRateByTopic(ctx context.Context, teamID int64, startMs, endMs int64) ([]TopicCounterRow, error)
}

type Service struct {
	repo publishRateRepo
}

func NewService(repo publishRateRepo) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetProduceRateByTopic(ctx context.Context, teamID int64, startMs, endMs int64) ([]TopicRatePoint, error) {
	rows, err := s.repo.QueryPublishRateByTopic(ctx, teamID, startMs, endMs)
	if err != nil {
		return nil, err
	}
	folds := filter.FoldCounterRateByDim(rows,
		func(r TopicCounterRow) time.Time { return r.Timestamp },
		func(r TopicCounterRow) string { return r.Topic },
		func(r TopicCounterRow) float64 { return r.Value },
		startMs, endMs)
	out := make([]TopicRatePoint, len(folds))
	for i, fld := range folds {
		out[i] = TopicRatePoint{Timestamp: filter.FormatTime(fld.Ts), Topic: fld.Dim, RatePerSec: fld.Rate}
	}
	return out, nil
}

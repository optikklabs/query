package scores

import (
	"context"
	"errors"
	"fmt"
)

var errInvalidScore = errors.New("invalid score")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, tenantID int64, req CreateScoreRequest) error {
	if req.TraceID == "" || req.Name == "" {
		return fmt.Errorf("%w: traceId and name are required", errInvalidScore)
	}
	switch req.DataType {
	case "numeric", "boolean", "categorical":
	default:
		return fmt.Errorf("%w: dataType must be numeric, boolean or categorical", errInvalidScore)
	}
	if req.DataType != "categorical" && req.Value == nil {
		return fmt.Errorf("%w: value is required for numeric/boolean scores", errInvalidScore)
	}

	row, err := s.repo.LookupTraceContext(ctx, tenantID, req.TraceID)
	if err != nil {
		return err
	}
	row.TenantID = tenantID
	row.TraceID = req.TraceID
	row.SpanID = req.SpanID
	row.Name = req.Name
	row.DataType = req.DataType
	row.StringValue = req.StringValue
	row.Comment = req.Comment
	if req.Value != nil {
		row.Value = *req.Value
	}
	return s.repo.Insert(ctx, row)
}

func IsValidationError(err error) bool { return errors.Is(err, errInvalidScore) }

func (s *Service) Names(ctx context.Context, tenantID, startMs, endMs int64) (ScoreNamesResponse, error) {
	rows, err := s.repo.Names(ctx, tenantID, startMs, endMs)
	if err != nil {
		return ScoreNamesResponse{}, err
	}
	names := make([]ScoreName, len(rows))
	for i, r := range rows {
		names[i] = ScoreName{Name: r.Name, DataType: r.DataType}
	}
	return ScoreNamesResponse{Names: names}, nil
}

func (s *Service) Summary(ctx context.Context, tenantID, startMs, endMs int64) (ScoreSummaryResponse, error) {
	rows, err := s.repo.Summary(ctx, tenantID, startMs, endMs)
	if err != nil {
		return ScoreSummaryResponse{}, err
	}
	out := make([]ScoreSummary, len(rows))
	for i, r := range rows {
		out[i] = ScoreSummary{Name: r.Name, DataType: r.DataType, Count: r.Count, Mean: r.Mean}
	}
	return ScoreSummaryResponse{Summaries: out}, nil
}

func (s *Service) Timeseries(ctx context.Context, tenantID, startMs, endMs int64, name string) (ScoreTimeseriesResponse, error) {
	rows, err := s.repo.Timeseries(ctx, tenantID, startMs, endMs, name)
	if err != nil {
		return ScoreTimeseriesResponse{}, err
	}
	points := make([]Point, len(rows))
	for i, r := range rows {
		points[i] = Point{T: r.BucketAt.UnixMilli(), Value: r.Mean}
	}
	return ScoreTimeseriesResponse{Name: name, Points: points}, nil
}

var bucketLabels = [10]string{
	"0.0–0.1", "0.1–0.2", "0.2–0.3", "0.3–0.4", "0.4–0.5",
	"0.5–0.6", "0.6–0.7", "0.7–0.8", "0.8–0.9", "0.9–1.0",
}

func (s *Service) Distribution(ctx context.Context, tenantID, startMs, endMs int64, name string) (ScoreDistributionResponse, error) {
	rows, err := s.repo.Distribution(ctx, tenantID, startMs, endMs, name)
	if err != nil {
		return ScoreDistributionResponse{}, err
	}
	counts := [10]uint64{}
	for _, r := range rows {
		if int(r.Bucket) < len(counts) {
			counts[r.Bucket] = r.Count
		}
	}
	buckets := make([]DistributionBucket, len(bucketLabels))
	for i, label := range bucketLabels {
		buckets[i] = DistributionBucket{Label: label, Count: counts[i]}
	}
	return ScoreDistributionResponse{Name: name, Buckets: buckets}, nil
}

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

func (s *Service) Summary(ctx context.Context, tenantID, startMs, endMs int64) (ScoreSummaryResponse, error) {
	rows, err := s.repo.Summary(ctx, tenantID, startMs, endMs)
	if err != nil {
		return ScoreSummaryResponse{}, err
	}
	out := make([]ScoreSummary, len(rows))
	for i, r := range rows {
		out[i] = ScoreSummary(r)
	}
	return ScoreSummaryResponse{Summaries: out}, nil
}

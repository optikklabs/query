package scores

import (
	"context"
	"slices"

	"github.com/optikklabs/query/internal/shared/errorcode"
)

var dataTypes = []string{"numeric", "boolean", "categorical"}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, tenantID int64, req CreateScoreRequest) error {
	switch {
	case req.TraceID == "" || req.Name == "":
		return errorcode.ValidationError{Msg: "traceId and name are required"}
	case !slices.Contains(dataTypes, req.DataType):
		return errorcode.ValidationError{Msg: "dataType must be numeric, boolean or categorical"}
	case req.DataType != "categorical" && req.Value == nil:
		return errorcode.ValidationError{Msg: "value is required for numeric/boolean scores"}
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

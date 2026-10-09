package evaluators

import (
	"cmp"
	"context"
	"database/sql"
	"slices"
	"strings"

	"github.com/optikklabs/query/internal/shared/nullable"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

var (
	ErrNotFound  = errorcode.NotFoundError{Msg: "evaluator not found"}
	errDuplicate = errorcode.ConflictError{Msg: "an evaluator with this name already exists"}
)

var (
	targets   = []string{"traces", "generations"}
	dataTypes = []string{"numeric", "boolean", "categorical"}
)

func (s *Service) List(ctx context.Context, tenantID, startMs, endMs int64) ([]Evaluator, error) {
	rows, err := s.repo.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.ScoreName)
	}
	aggs, err := s.repo.ScoreAggregates(ctx, tenantID, startMs, endMs, names)
	if err != nil {
		return nil, err
	}
	out := make([]Evaluator, 0, len(rows))
	for _, row := range rows {
		ev := toEvaluator(row)
		if a, ok := aggs[row.ScoreName]; ok {
			ev.Analytics = EvaluatorMetrics{Count: a.Count, MeanValue: a.Mean}
		}
		out = append(out, ev)
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, tenantID, userID int64, req UpsertRequest) (Evaluator, error) {
	args, err := buildArgs(tenantID, req, insertArgs{Target: "generations", SamplingPct: 100, DataType: "numeric", Enabled: true})
	if err != nil {
		return Evaluator{}, err
	}
	args.CreatedBy = sql.NullInt64{Valid: true, Int64: userID}
	id, err := s.repo.Create(ctx, args)
	if err != nil {
		return Evaluator{}, dbutil.DuplicateAs(err, errDuplicate)
	}
	return s.get(ctx, tenantID, id)
}

func (s *Service) Update(ctx context.Context, tenantID, id int64, req UpsertRequest) (Evaluator, error) {
	cur, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return Evaluator{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	args, err := buildArgs(tenantID, req, fromRow(cur))
	if err != nil {
		return Evaluator{}, err
	}
	if err := s.repo.Update(ctx, tenantID, id, args); err != nil {
		return Evaluator{}, dbutil.DuplicateAs(dbutil.NoRowsAs(err, ErrNotFound), errDuplicate)
	}
	return s.get(ctx, tenantID, id)
}

func (s *Service) Delete(ctx context.Context, tenantID, id int64) error {
	return dbutil.NoRowsAs(s.repo.Delete(ctx, tenantID, id), ErrNotFound)
}

func (s *Service) get(ctx context.Context, tenantID, id int64) (Evaluator, error) {
	row, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return Evaluator{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	return toEvaluator(row), nil
}

func buildArgs(tenantID int64, req UpsertRequest, base insertArgs) (insertArgs, error) {
	base.TenantID = tenantID
	if name := strings.TrimSpace(req.Name); name != "" {
		base.Name = name
	}
	if base.Name == "" {
		return insertArgs{}, errorcode.ValidationError{Msg: "name is required"}
	}
	if sn := strings.TrimSpace(req.ScoreName); sn != "" {
		base.ScoreName = sn
	}
	if base.ScoreName == "" {
		return insertArgs{}, errorcode.ValidationError{Msg: "scoreName is required"}
	}
	if req.Target != "" {
		if !slices.Contains(targets, req.Target) {
			return insertArgs{}, errorcode.ValidationError{Msg: "target must be traces or generations"}
		}
		base.Target = req.Target
	}
	if req.DataType != "" {
		if !slices.Contains(dataTypes, req.DataType) {
			return insertArgs{}, errorcode.ValidationError{Msg: "dataType must be numeric, boolean or categorical"}
		}
		base.DataType = req.DataType
	}
	if req.SamplingPct != nil {
		if *req.SamplingPct < 0 || *req.SamplingPct > 100 {
			return insertArgs{}, errorcode.ValidationError{Msg: "samplingPct must be between 0 and 100"}
		}
		base.SamplingPct = *req.SamplingPct
	}
	if req.Enabled != nil {
		base.Enabled = *req.Enabled
	}
	if req.Categories != nil {
		base.Categories = req.Categories
	}
	if strings.TrimSpace(req.JudgeModel) != "" {
		base.JudgeModel = sql.NullString{Valid: true, String: strings.TrimSpace(req.JudgeModel)}
	}
	if strings.TrimSpace(req.PromptTemplate) != "" {
		base.PromptTemplate = sql.NullString{Valid: true, String: req.PromptTemplate}
	}
	return base, nil
}

func fromRow(row evaluatorRow) insertArgs {
	return insertArgs{
		Name:           row.Name,
		ScoreName:      row.ScoreName,
		JudgeModel:     row.JudgeModel,
		Target:         row.Target,
		SamplingPct:    row.SamplingPct,
		DataType:       row.DataType,
		Categories:     row.Categories,
		PromptTemplate: row.PromptTemplate,
		Enabled:        row.Enabled,
	}
}

func toEvaluator(row evaluatorRow) Evaluator {
	return Evaluator{
		ID:             row.ID,
		Name:           row.Name,
		ScoreName:      row.ScoreName,
		JudgeModel:     row.JudgeModel.String,
		Target:         row.Target,
		SamplingPct:    row.SamplingPct,
		DataType:       row.DataType,
		Categories:     nullable.OrEmpty(row.Categories),
		PromptTemplate: row.PromptTemplate.String,
		Enabled:        row.Enabled,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      cmp.Or(row.UpdatedAt.Time, row.CreatedAt),
	}
}

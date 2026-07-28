package evaluators

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

var ErrNotFound = errors.New("evaluator not found")

type ErrValidation struct{ Msg string }

func (e ErrValidation) Error() string { return e.Msg }

var validTarget = map[string]struct{}{"traces": {}, "generations": {}}
var validDataType = map[string]struct{}{"numeric": {}, "boolean": {}, "categorical": {}}

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
	if userID > 0 {
		args.CreatedBy = sql.NullInt64{Valid: true, Int64: userID}
	}
	id, err := s.repo.Create(ctx, args)
	if err != nil {
		return Evaluator{}, err
	}
	return s.get(ctx, tenantID, id)
}

func (s *Service) Update(ctx context.Context, tenantID, id int64, req UpsertRequest) (Evaluator, error) {
	cur, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return Evaluator{}, mapNotFound(err)
	}
	args, err := buildArgs(tenantID, req, fromRow(cur))
	if err != nil {
		return Evaluator{}, err
	}
	if err := s.repo.Update(ctx, tenantID, id, args); err != nil {
		return Evaluator{}, mapNotFound(err)
	}
	return s.get(ctx, tenantID, id)
}

func (s *Service) Delete(ctx context.Context, tenantID, id int64) error {
	return mapNotFound(s.repo.Delete(ctx, tenantID, id))
}

func (s *Service) get(ctx context.Context, tenantID, id int64) (Evaluator, error) {
	row, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return Evaluator{}, mapNotFound(err)
	}
	return toEvaluator(row), nil
}

func buildArgs(tenantID int64, req UpsertRequest, base insertArgs) (insertArgs, error) {
	base.TenantID = tenantID
	if name := strings.TrimSpace(req.Name); name != "" {
		base.Name = name
	}
	if base.Name == "" {
		return insertArgs{}, ErrValidation{Msg: "name is required"}
	}
	if sn := strings.TrimSpace(req.ScoreName); sn != "" {
		base.ScoreName = sn
	}
	if base.ScoreName == "" {
		return insertArgs{}, ErrValidation{Msg: "scoreName is required"}
	}
	if req.Target != "" {
		if _, ok := validTarget[req.Target]; !ok {
			return insertArgs{}, ErrValidation{Msg: "target must be traces or generations"}
		}
		base.Target = req.Target
	}
	if req.DataType != "" {
		if _, ok := validDataType[req.DataType]; !ok {
			return insertArgs{}, ErrValidation{Msg: "dataType must be numeric, boolean or categorical"}
		}
		base.DataType = req.DataType
	}
	if req.SamplingPct != nil {
		if *req.SamplingPct < 0 || *req.SamplingPct > 100 {
			return insertArgs{}, ErrValidation{Msg: "samplingPct must be between 0 and 100"}
		}
		base.SamplingPct = *req.SamplingPct
	}
	if req.Enabled != nil {
		base.Enabled = *req.Enabled
	}
	if req.Categories != nil {
		base.CategoriesJSON = marshalStrings(req.Categories)
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
	a := insertArgs{
		Name:           row.Name,
		ScoreName:      row.ScoreName,
		Target:         row.Target,
		SamplingPct:    row.SamplingPct,
		DataType:       row.DataType,
		CategoriesJSON: row.CategoriesJSON,
		Enabled:        row.Enabled,
	}
	if row.JudgeModel != nil {
		a.JudgeModel = sql.NullString{Valid: true, String: *row.JudgeModel}
	}
	if row.PromptTemplate != nil {
		a.PromptTemplate = sql.NullString{Valid: true, String: *row.PromptTemplate}
	}
	return a
}

func toEvaluator(row evaluatorRow) Evaluator {
	ev := Evaluator{
		ID:          row.ID,
		Name:        row.Name,
		ScoreName:   row.ScoreName,
		Target:      row.Target,
		SamplingPct: row.SamplingPct,
		DataType:    row.DataType,
		Categories:  unmarshalStrings(row.CategoriesJSON),
		Enabled:     row.Enabled,
		CreatedAt:   row.CreatedAt,
	}
	if row.JudgeModel != nil {
		ev.JudgeModel = *row.JudgeModel
	}
	if row.PromptTemplate != nil {
		ev.PromptTemplate = *row.PromptTemplate
	}
	ev.UpdatedAt = row.CreatedAt
	if row.UpdatedAt != nil {
		ev.UpdatedAt = *row.UpdatedAt
	}
	return ev
}

func marshalStrings(in []string) []byte {
	if len(in) == 0 {
		return []byte("[]")
	}
	b, err := json.Marshal(in)
	if err != nil {
		return []byte("[]")
	}
	return b
}

func unmarshalStrings(raw []byte) []string {
	out := []string{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

func mapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

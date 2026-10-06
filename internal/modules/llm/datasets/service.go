package datasets

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/nullable"
)

const maxItemsPerRequest = 500

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

var ErrNotFound = errorcode.NotFoundError{Msg: "dataset not found"}

func (s *Service) List(ctx context.Context, tenantID int64) ([]DatasetSummary, error) {
	rows, err := s.repo.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]DatasetSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSummary(row))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, tenantID, id int64) (DatasetDetail, error) {
	row, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return DatasetDetail{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	items, err := s.repo.ListItems(ctx, id)
	if err != nil {
		return DatasetDetail{}, err
	}
	runs, err := s.repo.ListRuns(ctx, id)
	if err != nil {
		return DatasetDetail{}, err
	}
	detail := DatasetDetail{
		DatasetSummary: toSummary(row),
		Items:          make([]DatasetItem, 0, len(items)),
		Runs:           make([]RunSummary, 0, len(runs)),
	}
	for _, it := range items {
		detail.Items = append(detail.Items, toItem(it))
	}
	for _, run := range runs {
		detail.Runs = append(detail.Runs, toRunSummary(run))
	}
	return detail, nil
}

func (s *Service) Create(ctx context.Context, tenantID, userID int64, req CreateDatasetRequest) (DatasetDetail, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return DatasetDetail{}, errorcode.ValidationError{Msg: "name is required"}
	}
	var desc sql.NullString
	if d := strings.TrimSpace(req.Description); d != "" {
		desc = sql.NullString{Valid: true, String: d}
	}
	id, err := s.repo.Create(ctx, tenantID, userID, name, desc)
	if err != nil {
		return DatasetDetail{}, err
	}
	return s.Get(ctx, tenantID, id)
}

func (s *Service) Delete(ctx context.Context, tenantID, id int64) error {
	return dbutil.NoRowsAs(s.repo.Delete(ctx, tenantID, id), ErrNotFound)
}

func (s *Service) AddItems(ctx context.Context, tenantID, datasetID int64, req AddItemsRequest) (int, error) {
	if len(req.Items) == 0 {
		return 0, errorcode.ValidationError{Msg: "items must not be empty"}
	}
	if len(req.Items) > maxItemsPerRequest {
		return 0, errorcode.ValidationError{Msg: "too many items in one request"}
	}
	ok, err := s.repo.DatasetExists(ctx, tenantID, datasetID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNotFound
	}
	return s.repo.AddItems(ctx, tenantID, datasetID, req.Items)
}

func toSummary(row datasetRow) DatasetSummary {
	return DatasetSummary{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description.String,
		ItemCount:   row.ItemCount,
		RunCount:    row.RunCount,
		UpdatedAt:   cmp.Or(row.UpdatedAt.Time, row.CreatedAt),
	}
}

func toItem(row itemRow) DatasetItem {
	return DatasetItem{
		ID:             row.ID,
		Input:          rawJSON(row.InputJSON),
		ExpectedOutput: rawJSON(row.ExpectedOutputJSON),
		Metadata:       rawJSON(row.MetadataJSON),
		CreatedAt:      row.CreatedAt,
	}
}

func toRunSummary(row runRow) RunSummary {
	return RunSummary{
		ID:           row.ID,
		Name:         row.Name,
		Provider:     row.Provider,
		Model:        row.Model,
		Status:       row.Status,
		ItemCount:    row.ItemCount,
		AvgScores:    rawJSON(row.AvgScoresJSON),
		TotalCostUsd: row.TotalCostUsd,
		AvgLatencyMs: row.AvgLatencyMs,
		Error:        row.Error.String,
		CreatedAt:    row.CreatedAt,
		CompletedAt:  nullable.Ptr(row.CompletedAt.Time, row.CompletedAt.Valid),
	}
}

func toRunItem(row runItemRow) RunItem {
	return RunItem{
		DatasetItemID: row.DatasetItemID,
		Output:        rawJSON(row.OutputJSON),
		LatencyMs:     row.LatencyMs,
		CostUsd:       row.CostUsd,
		Scores:        rawJSON(row.ScoresJSON),
		Error:         row.Error.String,
	}
}

func rawJSON(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}

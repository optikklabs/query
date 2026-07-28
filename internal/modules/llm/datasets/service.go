package datasets

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/optikklabs/query/internal/shared/errorcode"
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
		return DatasetDetail{}, mapNotFound(err)
	}
	items, err := s.repo.ListItems(ctx, id)
	if err != nil {
		return DatasetDetail{}, err
	}
	runs, err := s.repo.ListRuns(ctx, id)
	if err != nil {
		return DatasetDetail{}, err
	}
	detail := DatasetDetail{DatasetSummary: toSummary(row)}
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
	return mapNotFound(s.repo.Delete(ctx, tenantID, id))
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

func (s *Service) GetRun(ctx context.Context, tenantID, runID int64) (RunDetail, error) {
	run, err := s.repo.GetRun(ctx, tenantID, runID)
	if err != nil {
		return RunDetail{}, mapNotFound(err)
	}
	items, err := s.repo.ListRunItems(ctx, runID)
	if err != nil {
		return RunDetail{}, err
	}
	detail := RunDetail{RunSummary: toRunSummary(run)}
	for _, it := range items {
		detail.Items = append(detail.Items, toRunItem(it))
	}
	return detail, nil
}

func toSummary(row datasetRow) DatasetSummary {
	sum := DatasetSummary{
		ID:        row.ID,
		Name:      row.Name,
		ItemCount: row.ItemCount,
		RunCount:  row.RunCount,
	}
	if row.Description != nil {
		sum.Description = *row.Description
	}
	sum.UpdatedAt = row.CreatedAt
	if row.UpdatedAt != nil {
		sum.UpdatedAt = *row.UpdatedAt
	}
	return sum
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
	sum := RunSummary{
		ID:           row.ID,
		Name:         row.Name,
		Provider:     row.Provider,
		Model:        row.Model,
		Status:       row.Status,
		ItemCount:    row.ItemCount,
		AvgScores:    rawJSON(row.AvgScoresJSON),
		TotalCostUsd: row.TotalCostUsd,
		AvgLatencyMs: row.AvgLatencyMs,
		CreatedAt:    row.CreatedAt,
		CompletedAt:  row.CompletedAt,
	}
	if row.Error != nil {
		sum.Error = *row.Error
	}
	return sum
}

func toRunItem(row runItemRow) RunItem {
	it := RunItem{
		DatasetItemID: row.DatasetItemID,
		Output:        rawJSON(row.OutputJSON),
		LatencyMs:     row.LatencyMs,
		CostUsd:       row.CostUsd,
		Scores:        rawJSON(row.ScoresJSON),
	}
	if row.Error != nil {
		it.Error = *row.Error
	}
	return it
}

func rawJSON(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}

func mapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

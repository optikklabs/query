package notifications

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

func (s *Service) CreateTemplate(ctx context.Context, tenantID int64, req CreateTemplateRequest) (TemplateResponse, error) {
	row, err := buildTemplateRow(tenantID, req)
	if err != nil {
		return TemplateResponse{}, err
	}
	id, err := s.repo.CreateTemplate(ctx, row)
	if err != nil {
		return TemplateResponse{}, err
	}
	row.ID = id
	return toTemplateResponse(row), nil
}

func (s *Service) UpdateTemplate(ctx context.Context, tenantID, id int64, req UpdateTemplateRequest) (TemplateResponse, error) {
	row, err := buildTemplateRow(tenantID, req)
	if err != nil {
		return TemplateResponse{}, err
	}
	row.ID = id
	if err := s.repo.UpdateTemplate(ctx, id, tenantID, row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TemplateResponse{}, ErrNotFound
		}
		return TemplateResponse{}, err
	}
	return toTemplateResponse(row), nil
}

func (s *Service) DeleteTemplate(ctx context.Context, tenantID, id int64) error {
	if err := s.repo.DeleteTemplate(ctx, id, tenantID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Service) ListTemplates(ctx context.Context, tenantID int64) ([]TemplateResponse, error) {
	rows, err := s.repo.ListTemplates(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]TemplateResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toTemplateResponse(row))
	}
	return out, nil
}

func buildTemplateRow(tenantID int64, req CreateTemplateRequest) (models.TemplateRow, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return models.TemplateRow{}, errorcode.ValidationError{Msg: "name is required"}
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		return models.TemplateRow{}, errorcode.ValidationError{Msg: "body is required"}
	}
	desc := sql.NullString{}
	if d := strings.TrimSpace(req.Description); d != "" {
		desc = sql.NullString{Valid: true, String: d}
	}
	return models.TemplateRow{
		TenantID:    tenantID,
		Name:        name,
		Description: desc,
		Body:        body,
	}, nil
}

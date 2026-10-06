package notifications

import (
	"context"
	"database/sql"
	"strings"

	dbutil "github.com/optikklabs/query/internal/infra/database"
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
	return s.getTemplate(ctx, tenantID, id)
}

func (s *Service) UpdateTemplate(ctx context.Context, tenantID, id int64, req UpdateTemplateRequest) (TemplateResponse, error) {
	row, err := buildTemplateRow(tenantID, req)
	if err != nil {
		return TemplateResponse{}, err
	}
	if err := s.repo.UpdateTemplate(ctx, id, tenantID, row); err != nil {
		return TemplateResponse{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	return s.getTemplate(ctx, tenantID, id)
}

func (s *Service) getTemplate(ctx context.Context, tenantID, id int64) (TemplateResponse, error) {
	row, err := s.repo.GetTemplate(ctx, id, tenantID)
	if err != nil {
		return TemplateResponse{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	return toTemplateResponse(row), nil
}

func (s *Service) DeleteTemplate(ctx context.Context, tenantID, id int64) error {
	return dbutil.NoRowsAs(s.repo.DeleteTemplate(ctx, id, tenantID), ErrNotFound)
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

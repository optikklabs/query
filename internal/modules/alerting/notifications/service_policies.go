package notifications

import (
	"context"
	"encoding/json"
	"strings"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

func (s *Service) CreatePolicy(ctx context.Context, tenantID int64, req CreatePolicyRequest) (PolicyResponse, error) {
	row, err := buildPolicyRow(tenantID, req)
	if err != nil {
		return PolicyResponse{}, err
	}
	id, err := s.repo.CreatePolicy(ctx, row)
	if err != nil {
		return PolicyResponse{}, err
	}
	row.ID = id
	return toPolicyResponse(row), nil
}

func (s *Service) UpdatePolicy(ctx context.Context, tenantID, id int64, req UpdatePolicyRequest) (PolicyResponse, error) {
	row, err := buildPolicyRow(tenantID, req)
	if err != nil {
		return PolicyResponse{}, err
	}
	row.ID = id
	if err := s.repo.UpdatePolicy(ctx, id, tenantID, row); err != nil {
		return PolicyResponse{}, dbutil.NoRowsAs(err, ErrNotFound)
	}
	return toPolicyResponse(row), nil
}

func (s *Service) DeletePolicy(ctx context.Context, tenantID, id int64) error {
	return dbutil.NoRowsAs(s.repo.DeletePolicy(ctx, id, tenantID), ErrNotFound)
}

func (s *Service) ListPolicies(ctx context.Context, tenantID int64) ([]PolicyResponse, error) {
	rows, err := s.repo.ListPolicies(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]PolicyResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPolicyResponse(row))
	}
	return out, nil
}

func buildPolicyRow(tenantID int64, req CreatePolicyRequest) (models.PolicyRow, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return models.PolicyRow{}, errorcode.ValidationError{Msg: "name is required"}
	}
	dsl := strings.TrimSpace(req.MatchDSL)
	if dsl == "" {
		return models.PolicyRow{}, errorcode.ValidationError{Msg: "matchDsl is required"}
	}
	actions := req.Actions
	if len(actions) == 0 {
		actions = json.RawMessage("[]")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	position := 0
	if req.Position != nil {
		position = *req.Position
	}
	return models.PolicyRow{
		TenantID:    tenantID,
		Name:        name,
		MatchDSL:    dsl,
		ActionsJSON: actions,
		Enabled:     enabled,
		Position:    position,
	}, nil
}

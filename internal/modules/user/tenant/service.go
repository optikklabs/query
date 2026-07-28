package tenant

import (
	"context"

	"github.com/optikklabs/query/internal/modules/user/shared"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) RotateAPIKey(ctx context.Context, tenantID int64) (TenantResponse, error) {
	apiKey, err := shared.GenerateAPIKey()
	if err != nil {
		return TenantResponse{}, shared.NewInternalError("Failed to generate api key", err)
	}
	resp, err := s.setTenantAPIKey(ctx, tenantID, apiKey)
	if err != nil {
		return TenantResponse{}, err
	}
	resp.APIKey = apiKey
	return resp, nil
}

func (s *Service) RevokeAPIKey(ctx context.Context, tenantID int64) (TenantResponse, error) {
	sentinel, err := shared.GenerateRevokedKey()
	if err != nil {
		return TenantResponse{}, shared.NewInternalError("Failed to revoke api key", err)
	}
	return s.setTenantAPIKey(ctx, tenantID, sentinel)
}

func (s *Service) setTenantAPIKey(ctx context.Context, tenantID int64, apiKey string) (TenantResponse, error) {
	if err := s.repo.UpdateTenantAPIKey(ctx, tenantID, shared.HashAPIKey(apiKey), shared.APIKeyPrefix(apiKey)); err != nil {
		return TenantResponse{}, shared.NewInternalError("Failed to update api key", err)
	}
	tenant, err := s.repo.FindTenantByID(ctx, tenantID)
	if err != nil {
		return TenantResponse{}, shared.NewInternalError("Failed to load tenant", err)
	}
	return toTenantResponse(tenant), nil
}

func toTenantResponse(tenant shared.TenantRecord) TenantResponse {
	return TenantResponse{
		ID:           tenant.ID,
		Name:         tenant.Name,
		Active:       tenant.Active,
		APIKeyPrefix: tenant.APIKeyPrefix,
		CreatedAt:    tenant.CreatedAt,
	}
}

func (s *Service) DeactivateTenant(ctx context.Context, tenantID int64) (TenantResponse, error) {
	if err := s.repo.DeactivateTenant(ctx, tenantID); err != nil {
		return TenantResponse{}, shared.NewInternalError("Failed to update tenant status", err)
	}
	tenant, err := s.repo.FindTenantByID(ctx, tenantID)
	if err != nil {
		return TenantResponse{}, shared.NewInternalError("Failed to load tenant", err)
	}
	return toTenantResponse(tenant), nil
}

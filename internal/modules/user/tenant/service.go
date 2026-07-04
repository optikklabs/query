package tenant

import (
	"context"

	"github.com/optikklabs/query/internal/modules/user/shared"
)

// Service manages tenant API-key lifecycle.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// RotateAPIKey issues a fresh key for the tenant; the previous key stops working
// once ingest's positive cache (5m) expires.
func (s *Service) RotateAPIKey(ctx context.Context, tenantID int64) (TenantResponse, error) {
	apiKey, err := shared.GenerateAPIKey()
	if err != nil {
		return TenantResponse{}, shared.NewInternalError("Failed to generate api key", err)
	}
	return s.setTenantAPIKey(ctx, tenantID, apiKey)
}

// RevokeAPIKey disables ingest by replacing the key with an unusable sentinel;
// the tenant must rotate to obtain a live key again.
func (s *Service) RevokeAPIKey(ctx context.Context, tenantID int64) (TenantResponse, error) {
	sentinel, err := shared.GenerateRevokedKey()
	if err != nil {
		return TenantResponse{}, shared.NewInternalError("Failed to revoke api key", err)
	}
	return s.setTenantAPIKey(ctx, tenantID, sentinel)
}

func (s *Service) setTenantAPIKey(ctx context.Context, tenantID int64, apiKey string) (TenantResponse, error) {
	if err := s.repo.UpdateTenantAPIKey(ctx, tenantID, apiKey); err != nil {
		return TenantResponse{}, shared.NewInternalError("Failed to update api key", err)
	}
	tenant, err := s.repo.FindTenantByID(tenantID)
	if err != nil {
		return TenantResponse{}, shared.NewInternalError("Failed to load tenant", err)
	}
	return toTenantResponse(tenant), nil
}

func toTenantResponse(tenant shared.TenantRecord) TenantResponse {
	return TenantResponse{
		ID:        tenant.ID,
		Name:      tenant.Name,
		Active:    tenant.Active,
		APIKey:    tenant.APIKey,
		CreatedAt: tenant.CreatedAt,
	}
}

// ActivateTenant marks the tenant as active.
func (s *Service) ActivateTenant(ctx context.Context, tenantID int64) (TenantResponse, error) {
	return s.setTenantActive(ctx, tenantID, true)
}

// DeactivateTenant marks the tenant as inactive. Sessions and ingest
// expire naturally — no cascade.
func (s *Service) DeactivateTenant(ctx context.Context, tenantID int64) (TenantResponse, error) {
	return s.setTenantActive(ctx, tenantID, false)
}

func (s *Service) setTenantActive(ctx context.Context, tenantID int64, active bool) (TenantResponse, error) {
	if err := s.repo.UpdateTenantActive(ctx, tenantID, active); err != nil {
		return TenantResponse{}, shared.NewInternalError("Failed to update tenant status", err)
	}
	tenant, err := s.repo.FindTenantByID(tenantID)
	if err != nil {
		return TenantResponse{}, shared.NewInternalError("Failed to load tenant", err)
	}
	return toTenantResponse(tenant), nil
}

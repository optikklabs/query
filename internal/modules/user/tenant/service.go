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
// once ingest's short positive cache expires. The response is the ONLY place
// the raw key ever appears — only its hash is stored, so a lost key cannot
// be recovered, only rotated again.
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

// RevokeAPIKey disables ingest by replacing the key with an unusable sentinel;
// the tenant must rotate to obtain a live key again.
func (s *Service) RevokeAPIKey(ctx context.Context, tenantID int64) (TenantResponse, error) {
	sentinel, err := shared.GenerateRevokedKey()
	if err != nil {
		return TenantResponse{}, shared.NewInternalError("Failed to revoke api key", err)
	}
	return s.setTenantAPIKey(ctx, tenantID, sentinel)
}

// setTenantAPIKey stores only the key's hash and display prefix; the raw
// key never reaches the database.
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

// DeactivateTenant marks the tenant as inactive. Sessions and ingest
// expire naturally — no cascade.
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

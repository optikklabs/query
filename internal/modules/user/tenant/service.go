package tenant

import (
	"context"
	"fmt"

	"github.com/optikklabs/query/internal/config"
	"github.com/optikklabs/query/internal/modules/user/shared"
)

// Header the ingest service reads the tenant API key from.
const apiKeyHeaderName = "x-api-key"

type Service struct {
	repo      *Repository
	ingestion config.IngestionConfig
}

func NewService(repo *Repository, ingestion config.IngestionConfig) *Service {
	return &Service{repo: repo, ingestion: ingestion}
}

func (s *Service) IngestionEndpoints() IngestionEndpointsResponse {
	return IngestionEndpointsResponse{
		GRPC:       s.ingestion.PublicGRPCEndpoint,
		HTTP:       s.ingestion.PublicHTTPEndpoint,
		HeaderName: apiKeyHeaderName,
	}
}

func (s *Service) RotateAPIKey(ctx context.Context, tenantID int64) (TenantResponse, error) {
	apiKey, err := shared.GenerateAPIKey()
	if err != nil {
		return TenantResponse{}, fmt.Errorf("failed to generate api key: %w", err)
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
		return TenantResponse{}, fmt.Errorf("failed to revoke api key: %w", err)
	}
	return s.setTenantAPIKey(ctx, tenantID, sentinel)
}

func (s *Service) setTenantAPIKey(ctx context.Context, tenantID int64, apiKey string) (TenantResponse, error) {
	if err := s.repo.UpdateTenantAPIKey(ctx, tenantID, shared.HashAPIKey(apiKey), shared.APIKeyPrefix(apiKey)); err != nil {
		return TenantResponse{}, fmt.Errorf("failed to update api key: %w", err)
	}
	tenant, err := s.repo.FindTenantByID(ctx, tenantID)
	if err != nil {
		return TenantResponse{}, fmt.Errorf("failed to load tenant: %w", err)
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
		return TenantResponse{}, fmt.Errorf("failed to update tenant status: %w", err)
	}
	tenant, err := s.repo.FindTenantByID(ctx, tenantID)
	if err != nil {
		return TenantResponse{}, fmt.Errorf("failed to load tenant: %w", err)
	}
	return toTenantResponse(tenant), nil
}

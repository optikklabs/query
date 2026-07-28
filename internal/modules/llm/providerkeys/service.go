package providerkeys

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/optikklabs/query/internal/infra/secretbox"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

type Service struct {
	repo *Repository
	box  *secretbox.Box
}

func NewService(repo *Repository, box *secretbox.Box) *Service {
	return &Service{repo: repo, box: box}
}

var (
	ErrNotFound     = errorcode.NotFoundError{Msg: "provider key not found"}
	ErrNoEncryption = errors.New("provider key encryption is not configured")
)

var validProvider = map[string]struct{}{
	"openai": {}, "anthropic": {}, "mistral": {},
}

func (s *Service) List(ctx context.Context, tenantID int64) ([]ProviderKey, error) {
	rows, err := s.repo.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]ProviderKey, 0, len(rows))
	for _, row := range rows {
		out = append(out, ProviderKey(row))
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, tenantID, userID int64, req CreateRequest) (ProviderKey, error) {
	if s.box == nil {
		return ProviderKey{}, ErrNoEncryption
	}
	if _, ok := validProvider[req.Provider]; !ok {
		return ProviderKey{}, errorcode.ValidationError{Msg: "provider must be openai, anthropic or mistral"}
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		return ProviderKey{}, errorcode.ValidationError{Msg: "label is required"}
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if len(apiKey) < 8 {
		return ProviderKey{}, errorcode.ValidationError{Msg: "apiKey looks too short to be valid"}
	}
	ct, nonce, err := s.box.Seal([]byte(apiKey))
	if err != nil {
		return ProviderKey{}, err
	}
	a := insertArgs{
		TenantID:   tenantID,
		Provider:   req.Provider,
		Label:      label,
		Ciphertext: ct,
		Nonce:      nonce,
		Last4:      apiKey[len(apiKey)-4:],
	}
	if userID > 0 {
		a.CreatedBy = sql.NullInt64{Valid: true, Int64: userID}
	}
	id, err := s.repo.Create(ctx, a)
	if err != nil {
		return ProviderKey{}, err
	}
	row, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return ProviderKey{}, err
	}
	return ProviderKey(row), nil
}

func (s *Service) Delete(ctx context.Context, tenantID, id int64) error {
	if errors.Is(s.repo.Delete(ctx, tenantID, id), sql.ErrNoRows) {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ResolveKey(ctx context.Context, tenantID int64, provider string) (string, error) {
	if s.box == nil {
		return "", ErrNoEncryption
	}
	row, err := s.repo.Secret(ctx, tenantID, provider)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	plain, err := s.box.Open(row.Ciphertext, row.Nonce)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

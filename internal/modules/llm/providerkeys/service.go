package providerkeys

import (
	"context"
	"database/sql"
	"strings"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/llmproviders"
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
	ErrNotFound = errorcode.NotFoundError{Msg: "provider key not found"}
	ErrNoKey    = errorcode.ValidationError{Msg: "no provider key is configured for this provider"}

	errDuplicate = errorcode.ConflictError{Msg: "a key with this provider and label already exists"}
)

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
	if err := llmproviders.ValidateProvider(req.Provider); err != nil {
		return ProviderKey{}, err
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
	a.CreatedBy = sql.NullInt64{Valid: true, Int64: userID}
	id, err := s.repo.Create(ctx, a)
	if err != nil {
		return ProviderKey{}, dbutil.DuplicateAs(err, errDuplicate)
	}
	row, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return ProviderKey{}, err
	}
	return ProviderKey(row), nil
}

func (s *Service) Delete(ctx context.Context, tenantID, id int64) error {
	return dbutil.NoRowsAs(s.repo.Delete(ctx, tenantID, id), ErrNotFound)
}

func (s *Service) ResolveKey(ctx context.Context, tenantID int64, provider string) (string, error) {
	row, err := s.repo.Secret(ctx, tenantID, provider)
	if err != nil {
		return "", dbutil.NoRowsAs(err, ErrNoKey)
	}
	plain, err := s.box.Open(row.Ciphertext, row.Nonce)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

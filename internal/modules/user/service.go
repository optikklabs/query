package user

import (
	"github.com/optikklabs/query/internal/infra/token"
)

// Service manages user, team, and auth-related operations.
type Service struct {
	repo   *Repository
	tokens *token.Service
}

func NewService(repo *Repository, tokens *token.Service) *Service {
	return &Service{
		repo:   repo,
		tokens: tokens,
	}
}

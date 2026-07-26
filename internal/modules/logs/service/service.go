// Package service holds the logs domain's business logic: it folds repository
// row types into the API models and owns pagination, suggestion routing, and
// facet assembly.
package service

import (
	"github.com/optikklabs/query/internal/modules/logs/repository"
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service { return &Service{repo: repo} }

// IsSuggestableScalarField reports whether a field has value suggestions. It is
// re-exported here so the handler can validate a suggest request without
// importing the repository, which would break the layering.
func IsSuggestableScalarField(field string) bool {
	return repository.IsSuggestableScalarField(field)
}

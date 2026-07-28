package service

import (
	"github.com/optikklabs/query/internal/modules/logs/repository"
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service { return &Service{repo: repo} }

func IsSuggestableScalarField(field string) bool {
	return repository.IsSuggestableScalarField(field)
}

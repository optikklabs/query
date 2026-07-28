package service

import (
	"github.com/optikklabs/query/internal/modules/saturation/database/repository"
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

const (
	DefaultPatternLimit    = repository.DefaultPatternLimit
	DefaultExecutionsLimit = repository.DefaultExecutionsLimit
)

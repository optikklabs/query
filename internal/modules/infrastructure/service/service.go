package service

import (
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service { return &Service{repo: repo} }

func ptr(v float64) *float64 { return &v }

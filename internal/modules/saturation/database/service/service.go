// Package service holds the business logic for the datastore saturation
// pages: it drives the repository and folds raw rows into API models.
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

// Page-size defaults, re-exported so the handler can reach them without
// importing the repository directly.
const (
	DefaultPatternLimit    = repository.DefaultPatternLimit
	DefaultExecutionsLimit = repository.DefaultExecutionsLimit
)

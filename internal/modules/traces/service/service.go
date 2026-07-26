// Package service holds the trace detail domain's business logic: folding
// span rows into the API models, reconciling structured events with exception
// columns, and the two graph walks (critical path, error path).
package service

import (
	"github.com/optikklabs/query/internal/modules/traces/repository"
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service { return &Service{repo: repo} }

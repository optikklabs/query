// Package service holds the Kafka saturation domain's business logic: the
// explorer reads pass straight through, while the topology folds edge rows
// into the producers->topics->consumers graph.
package service

import (
	"github.com/optikklabs/query/internal/modules/saturation/kafka/repository"
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service { return &Service{repo: repo} }

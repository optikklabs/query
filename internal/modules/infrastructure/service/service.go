package service

import (
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
	"github.com/optikklabs/query/internal/shared/errorcode"
)

var errUnknownMetricGroup = errorcode.ValidationError{Msg: "unknown metric group"}

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service { return &Service{repo: repo} }

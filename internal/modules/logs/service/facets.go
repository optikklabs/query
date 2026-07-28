package service

import (
	"context"

	"github.com/optikklabs/query/internal/modules/logs/filter"
	"github.com/optikklabs/query/internal/modules/logs/models"
)

func (s *Service) Facets(ctx context.Context, f filter.Filters) (models.Facets, error) {
	rows, err := s.repo.Facets(ctx, f)
	if err != nil {
		return models.Facets{}, err
	}
	fc := models.Facets{Severity: models.SeverityLabels}
	for _, r := range rows {
		fv := models.FacetValue{Value: r.Value, Count: r.Count}
		switch r.Dim {
		case "service":
			fc.Service = append(fc.Service, fv)
		case "host":
			fc.Host = append(fc.Host, fv)
		case "pod":
			fc.Pod = append(fc.Pod, fv)
		case "environment":
			fc.Environment = append(fc.Environment, fv)
		}
	}
	return fc, nil
}

func (s *Service) FacetsResponse(ctx context.Context, f filter.Filters) (models.FacetsResponse, error) {
	fc, err := s.Facets(ctx, f)
	if err != nil {
		return models.FacetsResponse{}, err
	}
	return models.FacetsResponse{Facets: fc}, nil
}

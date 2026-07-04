package onboarding

import (
	"context"
	"time"
)

const statusReady = "ready"

type repository interface {
	TeamStatus(ctx context.Context, teamID int64) (TeamStatus, error)
	FirstSeen(ctx context.Context, table string, teamID int64) (*time.Time, error)
}

type Service struct {
	repo repository
}

func NewService(repo repository) *Service { return &Service{repo: repo} }

// Status reports provisioning state plus first-data marks for one team.
func (s *Service) Status(ctx context.Context, teamID int64) (StatusResponse, error) {
	team, err := s.repo.TeamStatus(ctx, teamID)
	if err != nil {
		return StatusResponse{}, err
	}
	resp := StatusResponse{
		Provisioned: team.Status == statusReady,
		Status:      team.Status,
		Slug:        team.Slug,
		APIKey:      team.APIKey,
	}
	if resp.FirstSpanAt, err = s.repo.FirstSeen(ctx, "spans", teamID); err != nil {
		return StatusResponse{}, err
	}
	if resp.FirstLogAt, err = s.repo.FirstSeen(ctx, "logs", teamID); err != nil {
		return StatusResponse{}, err
	}
	if resp.FirstMetricAt, err = s.repo.FirstSeen(ctx, "metrics", teamID); err != nil {
		return StatusResponse{}, err
	}
	return resp, nil
}

package provisioner

import (
	"context"
	"fmt"
	"log/slog"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	namespace = "optikk"

	statusReady = "ready"
	// statusError parks teams whose slug/key can never render; no retry.
	statusError = "error"
)

type repository interface {
	ListPending(ctx context.Context) ([]PendingTeam, error)
	SetStatus(ctx context.Context, teamID int64, status string) error
}

type applier interface {
	Apply(ctx context.Context, objs []*unstructured.Unstructured) error
	CollectorAvailable(ctx context.Context, namespace, name string) (bool, error)
}

// Service provisions per-tenant collectors for teams still marked pending.
type Service struct {
	repo    repository
	applier applier
}

func NewService(repo repository, applier applier) *Service {
	return &Service{repo: repo, applier: applier}
}

// Tick provisions every pending team; per-team failures stay pending and retry.
func (s *Service) Tick(ctx context.Context) error {
	teams, err := s.repo.ListPending(ctx)
	if err != nil {
		return err
	}
	for _, t := range teams {
		if err := s.provision(ctx, t); err != nil {
			slog.Warn("onboarding.provisioner: provision failed",
				slog.Int64("team_id", t.ID), slog.Any("error", err))
		}
	}
	return nil
}

// provision applies the tenant manifests, flipping to ready once the
// collector has an available replica so "provisioned" means "can send now".
func (s *Service) provision(ctx context.Context, t PendingTeam) error {
	instance := fmt.Sprintf("%s-%d", t.Slug, t.ID)
	objs, err := renderTenant(instance, t.APIKey)
	if err != nil {
		if setErr := s.repo.SetStatus(ctx, t.ID, statusError); setErr != nil {
			slog.Warn("onboarding.provisioner: set error status failed",
				slog.Int64("team_id", t.ID), slog.Any("error", setErr))
		}
		return err
	}
	if err := s.applier.Apply(ctx, objs); err != nil {
		return err
	}
	available, err := s.applier.CollectorAvailable(ctx, namespace, "otel-collector-"+instance)
	if err != nil {
		return err
	}
	if !available {
		return nil
	}
	if err := s.repo.SetStatus(ctx, t.ID, statusReady); err != nil {
		return err
	}
	slog.Info("onboarding.provisioner: tenant provisioned",
		slog.Int64("team_id", t.ID), slog.String("instance", instance))
	return nil
}

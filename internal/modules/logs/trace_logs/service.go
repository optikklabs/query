package trace_logs

import (
	"context"
	"time"

	"github.com/optikklabs/query/internal/modules/logs/shared/models"
	"github.com/optikklabs/query/internal/shared/tracewindow"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// GetByTraceID resolves all logs for a (tenant_id, trace_id) pair.
//
// The scan is bounded by the trace's day-partitions rather than its exact span
// lifetime: a log is legitimately emitted just before the first span opens or
// after the root span closes. A trace with no spans at all is still valid — its
// spans may have been sampled out — so an unresolved trace falls back to the
// retention window instead of returning nothing.
func (s *Service) GetByTraceID(ctx context.Context, tenantID int64, traceID string, limit int) ([]models.Log, error) {
	w, ok, err := s.repo.ResolveWindow(ctx, tenantID, traceID)
	if err != nil {
		return nil, err
	}
	if ok {
		w = w.Days()
	} else {
		w = tracewindow.RetentionFallback(time.Now())
	}
	rows, err := s.repo.FetchLogsByTrace(ctx, tenantID, traceID, limit, w)
	if err != nil {
		return nil, err
	}
	return models.MapLogs(rows), nil
}

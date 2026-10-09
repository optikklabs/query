package service

import (
	"cmp"
	"context"
	"log/slog"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/optikklabs/query/internal/modules/saturation/database/models"
	"github.com/optikklabs/query/internal/modules/saturation/database/repository"
	"github.com/optikklabs/query/internal/shared/metrics"
)

func (s *Service) GetDatastoreSystems(ctx context.Context, tenantID, startMs, endMs int64) ([]models.DatastoreSystemRow, error) {
	var (
		spanRows []repository.SystemSummaryRaw
		conns    map[string]int64
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		rows, err := s.repo.GetSystemSummariesRaw(gctx, tenantID, startMs, endMs)
		if err != nil {
			return err
		}
		spanRows = rows
		return nil
	})
	g.Go(func() error {
		// Connection counts come from optional client metrics; without them
		// the systems still list, just with no connection count.
		c, err := s.repo.GetActiveConnectionsBySystem(gctx, tenantID, startMs, endMs)
		if err != nil {
			slog.WarnContext(gctx, "datastores: active connections lookup failed", slog.Any("error", err))
			return nil
		}
		conns = c
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return mapDatastoreSystems(spanRows, conns), nil
}

func mapDatastoreSystems(spanRows []repository.SystemSummaryRaw, conns map[string]int64) []models.DatastoreSystemRow {
	rows := make([]models.DatastoreSystemRow, 0, len(spanRows))
	seen := make(map[string]struct{}, len(spanRows))
	for _, r := range spanRows {
		queryCount := int64(r.QueryCount)
		errorCount := int64(r.ErrorCount)
		seen[r.DBSystem] = struct{}{}
		rows = append(rows, models.DatastoreSystemRow{
			System:            r.DBSystem,
			Category:          datastoreCategory(r.DBSystem),
			QueryCount:        queryCount,
			AvgLatencyMs:      r.AvgLatencyMs,
			P95LatencyMs:      r.P95Ms,
			ErrorRate:         metrics.Percentage(errorCount, queryCount),
			ActiveConnections: conns[r.DBSystem],
			Region:            r.Region,
			LastSeen:          r.LastSeen.Format(time.RFC3339),
		})
	}

	for system, active := range conns {
		if _, ok := seen[system]; ok {
			continue
		}
		rows = append(rows, models.DatastoreSystemRow{
			System:            system,
			Category:          datastoreCategory(system),
			ActiveConnections: active,
		})
	}

	slices.SortFunc(rows, func(a, b models.DatastoreSystemRow) int {
		return cmp.Or(cmp.Compare(b.QueryCount, a.QueryCount), strings.Compare(a.System, b.System))
	})
	return rows
}

func datastoreCategory(system string) string {
	if strings.EqualFold(strings.TrimSpace(system), "redis") {
		return "redis"
	}
	return "database"
}

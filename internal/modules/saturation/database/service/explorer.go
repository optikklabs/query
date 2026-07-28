package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/optikklabs/query/internal/modules/saturation/database/models"
	"github.com/optikklabs/query/internal/modules/saturation/database/repository"
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

		c, err := s.repo.GetActiveConnectionsBySystem(gctx, tenantID, startMs, endMs)
		if err == nil {
			conns = c
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

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
			P95LatencyMs:      float64(r.P95Ms),
			ErrorRate:         safeRatioPct(errorCount, queryCount),
			ActiveConnections: conns[r.DBSystem],
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

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].QueryCount == rows[j].QueryCount {
			return rows[i].System < rows[j].System
		}
		return rows[i].QueryCount > rows[j].QueryCount
	})

	return rows, nil
}

func datastoreCategory(system string) string {
	if strings.EqualFold(strings.TrimSpace(system), "redis") {
		return "redis"
	}
	return "database"
}

func safeRatioPct(numerator int64, denominator int64) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator) * 100
}

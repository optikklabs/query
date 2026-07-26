package cloud

import (
	"context"
	"time"

	"github.com/optikklabs/query/internal/shared/metrics"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// categoryOrder fixes the display order of category buckets.
var categoryOrder = []string{
	CategoryCompute, CategoryData, CategoryStorage,
	CategoryNetwork, CategoryStreaming, CategoryAI, CategoryOther,
}

func (s *Service) GetInventory(ctx context.Context, tenantID, startMs, endMs int64) ([]InventoryRow, error) {
	return s.repo.QueryProviderInventory(ctx, tenantID, startMs, endMs)
}

func (s *Service) GetCategories(ctx context.Context, tenantID, startMs, endMs int64) (map[string][]CategoryCount, error) {
	categories, err := s.repo.QueryProviderCategories(ctx, tenantID, startMs, endMs)
	if err != nil {
		return nil, err
	}
	return aggregateCategories(categories), nil
}

func (s *Service) GetHealth(ctx context.Context, tenantID, startMs, endMs int64) (map[string]HealthCounts, error) {
	health, err := s.repo.QueryProviderHealth(ctx, tenantID, startMs, endMs)
	if err != nil {
		return nil, err
	}
	return aggregateHealth(health), nil
}

func (s *Service) GetRestarts(ctx context.Context, tenantID, startMs, endMs int64) (map[string]uint64, error) {
	restarts, err := s.repo.QueryRestarts(ctx, tenantID, startMs, endMs)
	if err != nil {
		return nil, err
	}
	return indexRestarts(restarts), nil
}

func (s *Service) GetProviderPlatforms(ctx context.Context, tenantID int64, provider string, startMs, endMs int64) ([]PlatformService, error) {
	platforms, err := s.repo.QueryPlatformServices(ctx, tenantID, provider, startMs, endMs)
	if err != nil {
		return nil, err
	}
	out := make([]PlatformService, 0, len(platforms))
	for _, p := range platforms {
		out = append(out, PlatformService{
			Platform: p.Platform,
			Category: CategoryFor(p.Platform),
			Count:    int64(p.Count),
		})
	}
	return out, nil
}

func (s *Service) GetProviderAccounts(ctx context.Context, tenantID int64, provider string, startMs, endMs int64) ([]AccountBreakdown, error) {
	accounts, err := s.repo.QueryAccountBreakdown(ctx, tenantID, provider, startMs, endMs)
	if err != nil {
		return nil, err
	}
	out := make([]AccountBreakdown, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, AccountBreakdown{
			Account:   a.Account,
			Resources: int64(a.Resources),
			Nodes:     int64(a.Nodes),
			Pods:      int64(a.Pods),
		})
	}
	return out, nil
}

func (s *Service) GetProviderResources(ctx context.Context, tenantID int64, provider string, startMs, endMs int64) ([]AttentionResource, error) {
	resources, err := s.repo.QueryProviderResources(ctx, tenantID, provider, startMs, endMs)
	if err != nil {
		return nil, err
	}
	out := make([]AttentionResource, 0, len(resources))
	for _, r := range resources {
		errorRate, avgLatency := metrics.REDDerivations(r.RequestCount, r.ErrorCount, r.DurationMsSum)
		out = append(out, AttentionResource{
			Entity:       r.Entity,
			Service:      r.Service,
			Region:       r.Region,
			Platform:     r.Platform,
			Health:       classifyHealth(errorRate),
			ErrorRate:    errorRate,
			AvgLatencyMs: avgLatency,
			RequestCount: int64(r.RequestCount),
		})
	}
	return out, nil
}

// aggregateCategories folds per-platform counts into ordered category buckets.
func aggregateCategories(rows []CategoryRow) map[string][]CategoryCount {
	totals := map[string]map[string]int64{}
	for _, row := range rows {
		cat := CategoryFor(row.Platform)
		if totals[row.Provider] == nil {
			totals[row.Provider] = map[string]int64{}
		}
		totals[row.Provider][cat] += int64(row.Count)
	}
	out := map[string][]CategoryCount{}
	for provider, byCat := range totals {
		buckets := make([]CategoryCount, 0, len(byCat))
		for _, cat := range categoryOrder {
			if c, ok := byCat[cat]; ok {
				buckets = append(buckets, CategoryCount{Category: cat, Count: c})
			}
		}
		out[provider] = buckets
	}
	return out
}

// aggregateHealth classifies each entity and counts buckets per provider.
func aggregateHealth(rows []HealthRow) map[string]HealthCounts {
	out := map[string]HealthCounts{}
	for _, row := range rows {
		errorRate, _ := metrics.REDDerivations(row.RequestCount, row.ErrorCount, 0)
		counts := out[row.Provider]
		switch classifyHealth(errorRate) {
		case "unhealthy":
			counts.Unhealthy++
		case "degraded":
			counts.Degraded++
		default:
			counts.Healthy++
		}
		out[row.Provider] = counts
	}
	return out
}

func indexRestarts(rows []RestartRow) map[string]uint64 {
	out := make(map[string]uint64, len(rows))
	for _, row := range rows {
		out[row.Provider] = row.Restarts
	}
	return out
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

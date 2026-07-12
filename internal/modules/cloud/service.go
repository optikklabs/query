package cloud

import (
	"context"
	"time"
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

// GetOverview assembles the cross-provider landing payload from telemetry.
func (s *Service) GetOverview(ctx context.Context, tenantID, startMs, endMs int64) (CloudOverview, error) {
	inventory, err := s.repo.QueryProviderInventory(ctx, tenantID, startMs, endMs)
	if err != nil {
		return CloudOverview{}, err
	}
	categories, err := s.repo.QueryProviderCategories(ctx, tenantID, startMs, endMs)
	if err != nil {
		return CloudOverview{}, err
	}
	health, err := s.repo.QueryProviderHealth(ctx, tenantID, startMs, endMs)
	if err != nil {
		return CloudOverview{}, err
	}
	restarts, err := s.repo.QueryRestarts(ctx, tenantID, startMs, endMs)
	if err != nil {
		return CloudOverview{}, err
	}

	categoriesByProvider := aggregateCategories(categories)
	healthByProvider := aggregateHealth(health)
	restartsByProvider := indexRestarts(restarts)

	out := CloudOverview{Providers: make([]ProviderSummary, 0, len(inventory))}
	for _, inv := range inventory {
		h := healthByProvider[inv.Provider]
		summary := ProviderSummary{
			Provider:   inv.Provider,
			Accounts:   int64(inv.Accounts),
			Regions:    int64(inv.Regions),
			Nodes:      int64(inv.Nodes),
			Pods:       int64(inv.Pods),
			Resources:  int64(inv.Resources),
			Restarts:   int64(restartsByProvider[inv.Provider]),
			Categories: categoriesByProvider[inv.Provider],
			Health:     h,
			LastSeen:   formatTime(inv.LastSeen),
		}
		out.Providers = append(out.Providers, summary)

		out.TotalResources += summary.Resources
		out.TotalAccounts += summary.Accounts
		out.TotalRegions += summary.Regions
		out.TotalNodes += summary.Nodes
		out.TotalPods += summary.Pods
		out.Unhealthy += h.Unhealthy
		out.Degraded += h.Degraded
	}
	return out, nil
}

// GetProviderDetail assembles the per-provider drill-in payload.
func (s *Service) GetProviderDetail(ctx context.Context, tenantID int64, provider string, startMs, endMs int64) (CloudProviderDetail, error) {
	platforms, err := s.repo.QueryPlatformServices(ctx, tenantID, provider, startMs, endMs)
	if err != nil {
		return CloudProviderDetail{}, err
	}
	accounts, err := s.repo.QueryAccountBreakdown(ctx, tenantID, provider, startMs, endMs)
	if err != nil {
		return CloudProviderDetail{}, err
	}
	resources, err := s.repo.QueryProviderResources(ctx, tenantID, provider, startMs, endMs)
	if err != nil {
		return CloudProviderDetail{}, err
	}

	detail := CloudProviderDetail{
		Provider:  provider,
		Services:  make([]PlatformService, 0, len(platforms)),
		Accounts:  make([]AccountBreakdown, 0, len(accounts)),
		Resources: make([]AttentionResource, 0, len(resources)),
	}
	for _, p := range platforms {
		detail.Services = append(detail.Services, PlatformService{
			Platform: p.Platform,
			Category: CategoryFor(p.Platform),
			Count:    int64(p.Count),
		})
	}
	for _, a := range accounts {
		detail.Accounts = append(detail.Accounts, AccountBreakdown{
			Account:   a.Account,
			Resources: int64(a.Resources),
			Nodes:     int64(a.Nodes),
			Pods:      int64(a.Pods),
		})
	}
	for _, r := range resources {
		errorRate, avgLatency := redDerivations(r.RequestCount, r.ErrorCount, r.DurationMsSum)
		detail.Resources = append(detail.Resources, AttentionResource{
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
	return detail, nil
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
		errorRate, _ := redDerivations(row.RequestCount, row.ErrorCount, 0)
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

// redDerivations mirrors the nodes module: error-rate % and avg latency.
func redDerivations(reqCount, errCount uint64, durationMsSum float64) (errorRate, avgLatency float64) {
	if reqCount == 0 {
		return 0, 0
	}
	rc := float64(reqCount)
	return float64(errCount) * 100.0 / rc, durationMsSum / rc
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

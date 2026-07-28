package service

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/models"
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
	"github.com/optikklabs/query/internal/shared/httputil"
	"github.com/optikklabs/query/internal/shared/metrics"
)

func (s *Service) GetHosts(ctx context.Context, tenantID, startMs, endMs int64, serviceName string) ([]models.Host, error) {
	util, err := s.repo.QueryHostUtilization(ctx, tenantID, startMs, endMs)
	if err != nil {
		return nil, err
	}
	byHost, order := foldUtilization(util)

	if serviceName == "" {
		out := make([]models.Host, 0, len(order))
		for _, host := range order {
			out = append(out, byHost[host])
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Saturation > out[j].Saturation })
		return out, nil
	}

	spans, err := s.repo.QueryHostSpans(ctx, tenantID, startMs, endMs, serviceName)
	if err != nil {
		return nil, err
	}
	return enrichWithSpans(byHost, spans, endMs-startMs), nil
}

func foldUtilization(rows []repository.HostMetricRow) (map[string]models.Host, []string) {
	byMetric := map[string]map[string]float64{}
	order := []string{}
	for _, r := range rows {
		if _, ok := byMetric[r.Host]; !ok {
			byMetric[r.Host] = map[string]float64{}
			order = append(order, r.Host)
		}
		byMetric[r.Host][r.MetricName] = r.Value
	}

	byHost := make(map[string]models.Host, len(order))
	for _, host := range order {
		m := byMetric[host]
		cpu := valueOrZero(foldCPU(m))
		mem := valueOrZero(foldMem(m))
		disk := valueOrZero(foldDisk(m))
		sat := math.Max(cpu, math.Max(mem, disk))
		byHost[host] = models.Host{
			Host:       host,
			Subsystem:  subsystemForHost(host),
			CPU:        cpu,
			Mem:        mem,
			Disk:       disk,
			Saturation: sat,
			Tone:       toneForSaturation(sat),
		}
	}
	return byHost, order
}

func enrichWithSpans(byHost map[string]models.Host, spans []repository.HostSpansRow, windowMs int64) []models.Host {
	durationSec := float64(windowMs) / 1000.0
	if durationSec <= 0 {
		durationSec = 1
	}
	out := make([]models.Host, 0, len(spans))
	for _, row := range spans {
		h := byHost[row.Host]
		h.Host = row.Host

		total := int64(row.RequestCount)
		errs := int64(row.ErrorCount)
		errRate := metrics.PercentageInt(errs, total)
		rps := float64(total) / durationSec
		p99 := httputil.SanitizeFloat(float64(row.P99Ms))

		h.Zone = row.Zone
		h.RPS = &rps
		h.ErrorRate = &errRate
		h.P99Ms = &p99
		h.Status = classifyHost(errRate, float64(row.P99Ms))
		h.LastSeen = row.LastSeen.Format(time.RFC3339)
		h.RequestCount = total
		h.ErrorCount = errs
		out = append(out, h)
	}
	return out
}

func classifyHost(errRate, p99Ms float64) models.HostStatus {
	if errRate >= 0.10 || p99Ms >= 2000 {
		return models.HostError
	}
	if errRate >= 0.02 || p99Ms >= 1000 {
		return models.HostWarn
	}
	return models.HostHealthy
}

func subsystemForHost(host string) string {
	h := strings.ToLower(host)
	switch {
	case strings.HasPrefix(h, "kafka") || strings.Contains(h, "broker"):
		return models.SubsystemKafka
	case strings.HasPrefix(h, "pg") || strings.HasPrefix(h, "postgres") || strings.HasPrefix(h, "mysql") || strings.HasPrefix(h, "db"):
		return models.SubsystemDatabase
	default:
		return models.SubsystemOther
	}
}

func toneForSaturation(pct float64) string {
	switch {
	case pct >= 90:
		return "err"
	case pct >= 70:
		return "warn"
	default:
		return "ok"
	}
}

func foldCPU(m map[string]float64) *float64 {
	return averagePresent(m,
		infraconsts.MetricSystemCPUUtilization,
		infraconsts.MetricSystemCPUUsage,
		infraconsts.MetricProcessCPUUsage,
	)
}

func foldMem(m map[string]float64) *float64 {
	var values []float64
	if v, ok := m[infraconsts.MetricSystemMemoryUtilization]; ok {
		if nv := infraconsts.NormalizeUtilization(v); nv != nil {
			values = append(values, *nv)
		}
	}
	if max := m[infraconsts.MetricJVMMemoryMax]; max > 0 {
		values = append(values, infraconsts.PercentageMultiplier*m[infraconsts.MetricJVMMemoryUsed]/max)
	}
	return infraconsts.AverageUtilization(values)
}

func foldDisk(m map[string]float64) *float64 {
	return averagePresent(m, infraconsts.MetricSystemDiskUtilization)
}

func averagePresent(m map[string]float64, metricNames ...string) *float64 {
	var values []float64
	for _, name := range metricNames {
		if v, ok := m[name]; ok {
			if nv := infraconsts.NormalizeUtilization(v); nv != nil {
				values = append(values, *nv)
			}
		}
	}
	return infraconsts.AverageUtilization(values)
}

func valueOrZero(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

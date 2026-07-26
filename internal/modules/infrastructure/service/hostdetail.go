package service

import (
	"context"
	"math"
	"time"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/models"
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesdefs"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
)

// GetHostSeries reports known=false for a metric id outside the catalog, so
// the handler can answer 400 rather than an empty chart.
func (s *Service) GetHostSeries(ctx context.Context, tenantID int64, host, metricID string, startMs, endMs int64) ([]models.SeriesPoint, bool, error) {
	def, ok := seriesdefs.Host.Def(metricID)
	if !ok {
		return nil, false, nil
	}
	rows, err := s.repo.QueryHostSeries(ctx, tenantID, host, startMs, endMs, def)
	if err != nil {
		return nil, true, err
	}
	return scaleSeries(rows, def), true, nil
}

func (s *Service) GetHostOverview(ctx context.Context, tenantID int64, host string, startMs, endMs int64) (models.HostOverview, error) {
	meta, err := s.repo.QueryHostMeta(ctx, tenantID, host, startMs, endMs)
	if err != nil {
		return models.HostOverview{}, err
	}
	kpis, err := s.repo.QueryKPIs(ctx, tenantID, host, startMs, endMs)
	if err != nil {
		return models.HostOverview{}, err
	}

	out := models.HostOverview{
		Host:             host,
		Environments:     seriesgroup.EmptyIfNil(meta.Environments),
		Namespaces:       seriesgroup.EmptyIfNil(meta.Namespaces),
		AvailableMetrics: seriesgroup.EmptyIfNil(seriesdefs.Host.GroupsFor(meta.MetricNames)),
	}
	if !meta.LastSeen.IsZero() {
		out.LastSeen = meta.LastSeen.UTC().Format(time.RFC3339)
	}
	out.About = aboutFromMeta(meta)
	foldKPIs(kpis, &out)
	return out, nil
}

// scaleSeries applies the group's post-aggregation scale and normalizes a nil
// result to an empty slice so the JSON stays an array.
func scaleSeries(rows []models.SeriesPoint, def seriesgroup.Def) []models.SeriesPoint {
	if def.Scale != 1 {
		for i := range rows {
			rows[i].Value *= def.Scale
		}
	}
	if rows == nil {
		return []models.SeriesPoint{}
	}
	return rows
}

// aboutFromMeta builds the About panel payload; nil when the host has not
// reported any retained resource attributes yet.
func aboutFromMeta(meta repository.HostMetaRow) *models.HostAbout {
	about := models.HostAbout{
		OSType:        meta.OSType,
		OSDescription: meta.OSDescription,
		Arch:          meta.HostArch,
		HostID:        meta.HostID,
		CloudProvider: meta.CloudProvider,
		CloudPlatform: meta.CloudPlatform,
		CloudRegion:   meta.CloudRegion,
		CloudZone:     meta.CloudZone,
		K8SNodeName:   meta.K8SNodeName,
	}
	if about == (models.HostAbout{}) {
		return nil
	}
	return &about
}

// foldKPIs blends per-metric/state rows into the header KPI fields.
func foldKPIs(rows []repository.KPIRow, out *models.HostOverview) {
	var cpuIdle, cpuPlain, memUsed, memPlain *float64
	for _, row := range rows {
		v := row.Value
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			continue
		}
		switch row.MetricName {
		case infraconsts.MetricSystemCPUUtilization:
			switch row.State {
			case "idle":
				cpuIdle = ptr(v)
			case "":
				cpuPlain = ptr(v)
			}
		case infraconsts.MetricSystemMemoryUtilization:
			switch row.State {
			case "used":
				memUsed = ptr(v)
			case "":
				memPlain = ptr(v)
			}
		case infraconsts.MetricSystemFilesystemUtil:
			pct := toPercent(v)
			if out.DiskPct == nil || pct > *out.DiskPct {
				out.DiskPct = ptr(pct)
			}
		case infraconsts.MetricSystemCPULoadAvg1m:
			out.Load1m = ptr(v)
		case infraconsts.MetricSystemCPULoadAvg5m:
			out.Load5m = ptr(v)
		case infraconsts.MetricSystemCPULoadAvg15m:
			out.Load15m = ptr(v)
		case infraconsts.MetricSystemProcessCount:
			out.ProcessCount = ptr(v)
		}
	}
	// CPU busy: prefer 1 - idle (per-state agents); fall back to plain value.
	if cpuIdle != nil {
		out.CPUPct = ptr(toPercent(1 - *cpuIdle))
	} else if cpuPlain != nil {
		out.CPUPct = ptr(toPercent(*cpuPlain))
	}
	if memUsed != nil {
		out.MemoryPct = ptr(toPercent(*memUsed))
	} else if memPlain != nil {
		out.MemoryPct = ptr(toPercent(*memPlain))
	}
}

// toPercent normalizes 0..1 fractions to percentages, passing through
// values already expressed as percentages.
func toPercent(v float64) float64 {
	if v <= infraconsts.PercentageThreshold {
		return v * infraconsts.PercentageMultiplier
	}
	return v
}

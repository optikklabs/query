package hostdetail

import (
	"context"
	"math"
	"time"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetSeries(ctx context.Context, tenantID int64, host, metricID string, startMs, endMs int64) ([]SeriesPoint, bool, error) {
	def, ok := catalog.Def(metricID)
	if !ok {
		return nil, false, nil
	}
	rows, err := s.repo.QuerySeries(ctx, tenantID, host, startMs, endMs, def)
	if err != nil {
		return nil, true, err
	}
	if def.Scale != 1 {
		for i := range rows {
			rows[i].Value *= def.Scale
		}
	}
	if rows == nil {
		rows = []SeriesPoint{}
	}
	return rows, true, nil
}

func (s *Service) GetOverview(ctx context.Context, tenantID int64, host string, startMs, endMs int64) (HostOverview, error) {
	meta, err := s.repo.QueryHostMeta(ctx, tenantID, host, startMs, endMs)
	if err != nil {
		return HostOverview{}, err
	}
	kpis, err := s.repo.QueryKPIs(ctx, tenantID, host, startMs, endMs)
	if err != nil {
		return HostOverview{}, err
	}

	out := HostOverview{
		Host:             host,
		Environments:     seriesgroup.EmptyIfNil(meta.Environments),
		Namespaces:       seriesgroup.EmptyIfNil(meta.Namespaces),
		AvailableMetrics: seriesgroup.EmptyIfNil(catalog.GroupsFor(meta.MetricNames)),
	}
	if !meta.LastSeen.IsZero() {
		out.LastSeen = meta.LastSeen.UTC().Format(time.RFC3339)
	}
	out.About = aboutFromMeta(meta)
	foldKPIs(kpis, &out)
	return out, nil
}

// aboutFromMeta builds the About panel payload; nil when the host has not
// reported any retained resource attributes yet.
func aboutFromMeta(meta hostMetaRow) *HostAbout {
	about := HostAbout{
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
	if about == (HostAbout{}) {
		return nil
	}
	return &about
}

// foldKPIs blends per-metric/state rows into the header KPI fields.
func foldKPIs(rows []kpiRow, out *HostOverview) {
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

func ptr(v float64) *float64 { return &v }

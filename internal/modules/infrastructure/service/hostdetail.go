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
	"golang.org/x/sync/errgroup"
)

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
	var (
		meta repository.HostMetaRow
		kpis []repository.KPIRow
	)
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error
		meta, err = s.repo.QueryHostMeta(groupCtx, tenantID, host, startMs, endMs)
		return err
	})
	group.Go(func() error {
		var err error
		kpis, err = s.repo.QueryKPIs(groupCtx, tenantID, host, startMs, endMs)
		return err
	})
	if err := group.Wait(); err != nil {
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

func toPercent(v float64) float64 {
	if v <= infraconsts.PercentageThreshold {
		return v * infraconsts.PercentageMultiplier
	}
	return v
}

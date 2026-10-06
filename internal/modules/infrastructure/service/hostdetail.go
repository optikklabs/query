package service

import (
	"cmp"
	"context"

	"github.com/optikklabs/query/internal/shared/nullable"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/models"
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesdefs"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
	"golang.org/x/sync/errgroup"
)

func (s *Service) GetHostSeries(ctx context.Context, tenantID int64, host, metricID string, startMs, endMs int64) ([]models.SeriesPoint, error) {
	def, ok := seriesdefs.Host.Def(metricID)
	if !ok {
		return nil, errUnknownMetricGroup
	}
	rows, err := s.repo.QueryHostSeries(ctx, tenantID, host, startMs, endMs, def)
	if err != nil {
		return nil, err
	}
	return scaleSeries(rows, def), nil
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
		LastSeen:         meta.LastSeen,
		Environments:     nullable.OrEmpty(meta.Environments),
		Namespaces:       nullable.OrEmpty(meta.Namespaces),
		AvailableMetrics: nullable.OrEmpty(seriesdefs.Host.GroupsFor(meta.MetricNames)),
	}
	out.About = aboutFromMeta(meta)
	foldKPIs(kpis, &out)
	return out, nil
}

func scaleSeries(rows []models.SeriesPoint, def seriesgroup.Def) []models.SeriesPoint {
	for i := range rows {
		rows[i].Value *= def.Scale
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

// foldKPIs fills the host's KPI cards. Utilization metrics are ratios: CPU is
// read from its idle state (or the stateless series) and memory from its used
// state; load averages and process count are reported as-is.
func foldKPIs(rows []repository.KPIRow, out *models.HostOverview) {
	var cpuIdle, cpuPlain, memUsed, memPlain *float64
	for _, row := range rows {
		v := row.Value
		switch row.MetricName {
		case infraconsts.MetricSystemCPUUtilization:
			switch row.State {
			case "idle":
				cpuIdle = new(v)
			case "":
				cpuPlain = new(infraconsts.RatioPct(v))
			}
		case infraconsts.MetricSystemMemoryUtilization:
			switch row.State {
			case "used":
				memUsed = new(infraconsts.RatioPct(v))
			case "":
				memPlain = new(infraconsts.RatioPct(v))
			}
		case infraconsts.MetricSystemFilesystemUtil:
			if pct := infraconsts.RatioPct(v); out.DiskPct == nil || pct > *out.DiskPct {
				out.DiskPct = new(pct)
			}
		case infraconsts.MetricSystemCPULoadAvg1m:
			out.Load1m = new(v)
		case infraconsts.MetricSystemCPULoadAvg5m:
			out.Load5m = new(v)
		case infraconsts.MetricSystemCPULoadAvg15m:
			out.Load15m = new(v)
		case infraconsts.MetricSystemProcessCount:
			out.ProcessCount = new(v)
		}
	}
	if cpuIdle != nil {
		out.CPUPct = new(infraconsts.RatioPct(1 - *cpuIdle))
	} else {
		out.CPUPct = cpuPlain
	}
	out.MemoryPct = cmp.Or(memUsed, memPlain)
}

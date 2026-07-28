package redfleet

import (
	"context"
	"log/slog"
	"time"

	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/infra/utils"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/shared/metrics"
	"golang.org/x/sync/errgroup"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetFleetOverview(ctx context.Context, f REDFilters) (FleetOverviewResponse, error) {
	rows, err := s.repo.GetFleetREDMetrics(ctx, f)
	if err != nil {
		return FleetOverviewResponse{}, err
	}
	services := mapFleetServices(rows)
	return FleetOverviewResponse{
		Totals:   computeFleetTotals(fleetTotalRow(rows), len(services), f.StartMs, f.EndMs),
		Services: services,
	}, nil
}

func (s *Service) GetRequestAndErrorRateTimeSeries(ctx context.Context, f REDFilters) ([]ServicePerformancePoint, error) {
	rows, err := s.repo.GetRequestAndErrorRateTimeSeries(ctx, f)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	grainSec := grain.Seconds()
	return timebucket.FillGaps(f.StartMs, f.EndMs, grain, rows,
		func(r requestRateRawRow) time.Time { return r.BucketAt },
		func(t time.Time, row requestRateRawRow, ok bool) ServicePerformancePoint {
			pt := ServicePerformancePoint{Timestamp: t}
			if ok {
				pt.RequestCount = row.RequestCount
				pt.ErrorCount = row.ErrorCount
				pt.RPS = float64(row.RequestCount) / grainSec
				pt.ErrorRate = utils.SanitizeFloat(metrics.Percentage(row.ErrorCount, row.RequestCount))
			}
			return pt
		}), nil
}

func (s *Service) GetRequestRateTimeSeries(ctx context.Context, f REDFilters) (RequestRateSeries, error) {
	rows, err := s.repo.GetRequestRateTimeSeries(ctx, f)
	if err != nil {
		return RequestRateSeries{}, err
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	grainSec := grain.Seconds()

	type key struct {
		serviceName string
		timestamp   int64
	}
	rowMap := make(map[key]uint64, len(rows))
	seen := make(map[string]struct{})
	var services []string

	for _, row := range rows {
		ts := row.BucketAt.UTC().Truncate(grain).Unix()
		rowMap[key{serviceName: row.ServiceName, timestamp: ts}] = row.RequestCount
		if _, ok := seen[row.ServiceName]; !ok {
			seen[row.ServiceName] = struct{}{}
			services = append(services, row.ServiceName)
		}
	}

	buckets := timebucket.DenseBuckets(f.StartMs, f.EndMs, grain)
	out := RequestRateSeries{
		Timestamps: make([]int64, len(buckets)),
		Series:     make([]RequestRateEntry, len(services)),
	}
	for i, t := range buckets {
		out.Timestamps[i] = t.UnixMilli()
	}
	for i, svc := range services {
		rps := make([]float64, len(buckets))
		for j, t := range buckets {
			reqCount := rowMap[key{serviceName: svc, timestamp: t.Unix()}]
			rps[j] = utils.SanitizeFloat(float64(reqCount) / grainSec)
		}
		out.Series[i] = RequestRateEntry{ServiceName: svc, RPS: rps}
	}
	return out, nil
}

func (s *Service) GetStatusTimeSeries(ctx context.Context, f REDFilters) ([]StatusTimeSeriesPoint, error) {
	rows, err := s.repo.GetStatusTimeSeries(ctx, f)
	if err != nil {
		return nil, err
	}
	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	grainSec := grain.Seconds()

	byTs := make(map[int64]*StatusTimeSeriesPoint)
	for _, row := range rows {
		key := row.BucketAt.UTC().Truncate(grain).Unix()
		pt, ok := byTs[key]
		if !ok {
			pt = &StatusTimeSeriesPoint{Timestamp: row.BucketAt.UTC().Truncate(grain)}
			byTs[key] = pt
		}
		count := float64(row.RequestCount) / grainSec
		writeStatusCount(pt, row.StatusBucket, count)
	}

	var points []StatusTimeSeriesPoint
	for _, t := range timebucket.DenseBuckets(f.StartMs, f.EndMs, grain) {
		if pt, ok := byTs[t.Unix()]; ok {
			points = append(points, *pt)
		} else {
			points = append(points, StatusTimeSeriesPoint{Timestamp: t})
		}
	}
	return points, nil
}

func (s *Service) GetLatencyPercentilesTimeSeries(ctx context.Context, f REDFilters) ([]LatencyPercentilesPoint, error) {
	rows, err := s.repo.GetLatencyPercentilesTimeSeries(ctx, f)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	return timebucket.FillGaps(f.StartMs, f.EndMs, grain, rows,
		func(r latencyPercentilesTimeseriesRow) time.Time { return r.BucketAt },
		func(t time.Time, row latencyPercentilesTimeseriesRow, ok bool) LatencyPercentilesPoint {
			pt := LatencyPercentilesPoint{Timestamp: t}
			if ok {
				pt.P50Ms = utils.SanitizeFloat(float64(row.P50Ms))
				pt.P95Ms = utils.SanitizeFloat(float64(row.P95Ms))
				pt.P99Ms = utils.SanitizeFloat(float64(row.P99Ms))
			}
			return pt
		}), nil
}

func (s *Service) GetREDByEndpointTimeSeries(ctx context.Context, f REDFilters) (EndpointRateSeries, error) {
	rows, err := s.repo.GetREDByEndpointTimeSeries(ctx, f)
	if err != nil {
		return EndpointRateSeries{}, err
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	grainSec := float64(grain.Seconds())
	if grainSec <= 0 {
		grainSec = 60
	}

	type cell struct{ rps, errRate, p99 float64 }
	traffic := make(map[time.Time]map[string]cell, len(rows))
	var routes []string
	seenRoute := map[string]bool{}
	for _, row := range rows {
		bucket := row.BucketAt.UTC().Truncate(grain)
		if !seenRoute[row.HTTPRoute] {
			seenRoute[row.HTTPRoute] = true
			routes = append(routes, row.HTTPRoute)
		}
		var p99 float64
		errRate := metrics.Percentage(row.ErrorCount, row.RequestCount)
		if len(row.QS) >= 3 {
			p99 = utils.SanitizeFloat(row.QS[2])
		}
		if traffic[bucket] == nil {
			traffic[bucket] = map[string]cell{}
		}
		traffic[bucket][row.HTTPRoute] = cell{
			rps:     float64(row.RequestCount) / grainSec,
			errRate: errRate,
			p99:     p99,
		}
	}

	buckets := timebucket.DenseBuckets(f.StartMs, f.EndMs, grain)
	out := EndpointRateSeries{
		Timestamps: make([]int64, len(buckets)),
		Series:     make([]EndpointRateEntry, len(routes)),
	}
	for i, bucket := range buckets {
		out.Timestamps[i] = bucket.UnixMilli()
	}
	for i, route := range routes {
		entry := EndpointRateEntry{
			HTTPRoute: route,
			RPS:       make([]float64, len(buckets)),
			ErrorRate: make([]*float64, len(buckets)),
			P99Ms:     make([]*float64, len(buckets)),
		}
		for j, bucket := range buckets {
			if c, ok := traffic[bucket][route]; ok {
				errRate, p99 := c.errRate, c.p99
				entry.RPS[j], entry.ErrorRate[j], entry.P99Ms[j] = c.rps, &errRate, &p99
			}
		}
		out.Series[i] = entry
	}
	return out, nil
}

func (s *Service) GetTopEndpointsCombined(
	ctx context.Context, f REDFilters, limit int, cursorIn TopEndpointsCursor,
) (PaginatedEndpoints, error) {
	rows, err := s.repo.GetTopEndpointsCombined(ctx, f, limit+1, cursorIn)
	if err != nil {
		return PaginatedEndpoints{}, err
	}

	rows, pageInfo := cursor.Paginate(rows, limit, func(r topEndpointRow) string {
		return cursor.Encode(TopEndpointsCursor{TotalCount: r.TotalCount, OperationName: r.OperationName})
	})

	durationSec := float64(f.EndMs-f.StartMs) / 1000.0
	if durationSec <= 0 {
		durationSec = 1
	}

	results := make([]TopEndpoint, len(rows))
	for i, row := range rows {
		results[i] = toTopEndpoint(row, durationSec)
	}

	return PaginatedEndpoints{Results: results, PageInfo: pageInfo}, nil
}

func (s *Service) GetTopDBQueries(
	ctx context.Context, f REDFilters, limit int, cursorIn TopEndpointsCursor,
) (PaginatedDBQueries, error) {
	rows, err := s.repo.GetTopDBQueriesCombined(ctx, f, limit+1, cursorIn)
	if err != nil {
		return PaginatedDBQueries{}, err
	}

	rows, pageInfo := cursor.Paginate(rows, limit, func(r topDBQueryRow) string {
		return cursor.Encode(TopEndpointsCursor{TotalCount: r.TotalCount, OperationName: r.OperationName})
	})

	durationSec := float64(f.EndMs-f.StartMs) / 1000.0
	if durationSec <= 0 {
		durationSec = 1
	}

	results := make([]TopDBQuery, len(rows))
	for i, row := range rows {
		results[i] = toTopDBQuery(row, durationSec)
	}

	return PaginatedDBQueries{Results: results, PageInfo: pageInfo}, nil
}

func (s *Service) GetOperationBaseline(ctx context.Context, tenantID int64, startMs, endMs int64, serviceName, operationName string) (OperationBaseline, error) {
	row, err := s.repo.GetOperationBaseline(ctx, tenantID, startMs, endMs, serviceName, operationName)
	if err != nil {
		return OperationBaseline{}, err
	}
	return OperationBaseline{
		ServiceName:   serviceName,
		OperationName: operationName,
		P50Ms:         utils.SanitizeFloat(float64(row.P50Ms)),
		P95Ms:         utils.SanitizeFloat(float64(row.P95Ms)),
		P99Ms:         utils.SanitizeFloat(float64(row.P99Ms)),
		SpanCount:     int64(row.SpanCount),
	}, nil
}

func (s *Service) GetServiceSummary(ctx context.Context, f REDFilters) (ServiceSummaryResponse, error) {
	serviceName := f.SingleService()
	metricNames := []string{
		infraconsts.MetricSystemCPUUtilization,
		infraconsts.MetricSystemCPUUsage,
		infraconsts.MetricProcessCPUUsage,
		infraconsts.MetricJVMCPUUtilization,
		infraconsts.MetricSystemMemoryUtilization,
		infraconsts.MetricSystemDiskUtilization,
	}

	var (
		redRows []redMetricsRow
		sats    []serviceMetricRow
	)
	g, groupCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		redRows, err = s.repo.GetFleetREDMetrics(groupCtx, f)
		return err
	})
	g.Go(func() error {
		// Saturation is best-effort: degrade gracefully but never silently.
		rows, err := s.repo.GetServiceSaturationAggs(groupCtx, f.TenantID, f.StartMs, f.EndMs, serviceName, metricNames)
		if err != nil {
			slog.WarnContext(ctx, "service summary: saturation query failed, omitting saturation metrics",
				slog.String("service", serviceName),
				slog.Any("error", err),
			)
			return nil
		}
		sats = rows
		return nil
	})
	if err := g.Wait(); err != nil {
		return ServiceSummaryResponse{}, err
	}

	var redRow *redMetricsRow
	if len(redRows) > 0 {
		redRow = &redRows[0]
	}

	cpuVal, memVal, diskVal := extractSaturationAverages(sats)

	durationSec := float64(f.EndMs-f.StartMs) / 1000.0
	if durationSec <= 0 {
		durationSec = 1
	}

	reqCount, errCount, rps, errRate, p50, p95, p99 := extractREDMetrics(redRow, durationSec)

	return ServiceSummaryResponse{
		ServiceName:       serviceName,
		RequestCount:      reqCount,
		ErrorCount:        errCount,
		RPS:               utils.SanitizeFloat(rps),
		ErrorRate:         utils.SanitizeFloat(errRate),
		P50Ms:             p50,
		P95Ms:             p95,
		P99Ms:             p99,
		CPUUtilization:    utils.SanitizeFloat(cpuVal),
		MemoryUtilization: utils.SanitizeFloat(memVal),
		DiskUtilization:   utils.SanitizeFloat(diskVal),
	}, nil
}

func (s *Service) GetServiceSaturationTimeSeries(ctx context.Context, f REDFilters) ([]SaturationTimeSeriesPoint, error) {
	serviceName := f.SingleService()
	metricNames := []string{
		infraconsts.MetricSystemCPUUtilization,
		infraconsts.MetricSystemCPUUsage,
		infraconsts.MetricProcessCPUUsage,
		infraconsts.MetricJVMCPUUtilization,
	}

	rows, err := s.repo.GetServiceSaturationTimeSeries(ctx, f.TenantID, f.StartMs, f.EndMs, serviceName, metricNames)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	return timebucket.FillGaps(f.StartMs, f.EndMs, grain, rows,
		func(r saturationTimeSeriesRawRow) time.Time { return r.BucketAt },
		func(t time.Time, row saturationTimeSeriesRawRow, ok bool) SaturationTimeSeriesPoint {
			var val float64
			if ok {
				if normalized := infraconsts.NormalizeUtilization(row.Value); normalized != nil {
					val = *normalized
				}
			}
			return SaturationTimeSeriesPoint{
				Timestamp: t,
				Value:     utils.SanitizeFloat(val),
			}
		}), nil
}

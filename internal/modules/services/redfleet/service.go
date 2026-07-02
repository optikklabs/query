package redfleet

import (
	"context"
	"math"
	"time"

	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/infra/utils"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// GetFleetOverview queries fleet RED metrics once and returns both the
// aggregate totals and per-service breakdowns in a single response.
func (s *Service) GetFleetOverview(ctx context.Context, f REDFilters) (FleetOverviewResponse, error) {
	rows, err := s.repo.GetFleetREDMetrics(ctx, f)
	if err != nil {
		return FleetOverviewResponse{}, err
	}
	services := mapFleetServices(rows)
	return FleetOverviewResponse{
		Totals:   computeFleetTotals(services, f.StartMs, f.EndMs),
		Services: services,
	}, nil
}

// GetFleetServices returns one RED row per service across the whole fleet.
func (s *Service) GetFleetServices(ctx context.Context, f REDFilters) ([]ServiceREDMetric, error) {
	overview, err := s.GetFleetOverview(ctx, f)
	if err != nil {
		return nil, err
	}
	return overview.Services, nil
}



func mapFleetServices(rows []redMetricsRow) []ServiceREDMetric {
	services := make([]ServiceREDMetric, len(rows))
	for i, row := range rows {
		services[i] = ServiceREDMetric{
			ServiceName:  row.ServiceName,
			RequestCount: int64(row.TotalCount),
			ErrorCount:   int64(row.ErrorCount),
			AvgLatency:   utils.SanitizeFloat(float64(row.P50Ms)),
			P95Latency:   utils.SanitizeFloat(float64(row.P95Ms)),
			P99Latency:   utils.SanitizeFloat(float64(row.P99Ms)),
		}
	}
	return services
}

func computeFleetTotals(services []ServiceREDMetric, startMs, endMs int64) FleetTotals {
	durationSec := float64(endMs-startMs) / 1000.0
	if durationSec <= 0 {
		durationSec = 1
	}

	var totalCount, totalErrors int64
	var totalP50, totalP95, totalP99 float64
	for _, svc := range services {
		totalCount += svc.RequestCount
		totalErrors += svc.ErrorCount
		totalP50 += svc.AvgLatency
		totalP95 += svc.P95Latency
		totalP99 += svc.P99Latency
	}
	serviceCount := int64(len(services))

	avgErrorPct := 0.0
	if totalCount > 0 {
		avgErrorPct = float64(totalErrors) * 100.0 / float64(totalCount)
	}
	avgP50, avgP95, avgP99 := 0.0, 0.0, 0.0
	if serviceCount > 0 {
		avgP50 = totalP50 / float64(serviceCount)
		avgP95 = totalP95 / float64(serviceCount)
		avgP99 = totalP99 / float64(serviceCount)
	}
	return FleetTotals{
		ServiceCount:   serviceCount,
		TotalSpanCount: totalCount,
		TotalErrors:    totalErrors,
		TotalRPS:       utils.SanitizeFloat(float64(totalCount) / durationSec),
		AvgErrorPct:    utils.SanitizeFloat(avgErrorPct),
		AvgP50Ms:       utils.SanitizeFloat(avgP50),
		AvgP95Ms:       utils.SanitizeFloat(avgP95),
		AvgP99Ms:       utils.SanitizeFloat(avgP99),
	}
}




func (s *Service) GetRequestAndErrorRateTimeSeries(ctx context.Context, f REDFilters) ([]ServicePerformancePoint, error) {
	rows, err := s.repo.GetRequestAndErrorRateTimeSeries(ctx, f)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	grainSec := float64(grain.Seconds())
	if grainSec <= 0 {
		grainSec = 60
	}

	startTime := time.UnixMilli(f.StartMs).UTC().Truncate(grain)
	endTime := time.UnixMilli(f.EndMs).UTC().Truncate(grain)

	rowMap := make(map[int64]requestRateRawRow)
	for _, row := range rows {
		ts := row.BucketAt.UTC().Truncate(grain).Unix()
		rowMap[ts] = row
	}

	var points []ServicePerformancePoint
	for t := startTime; !t.After(endTime); t = t.Add(grain) {
		row, ok := rowMap[t.Unix()]
		var reqCount, errCount uint64
		var rps, errorPct float64
		if ok {
			reqCount = row.RequestCount
			errCount = row.ErrorCount
			rps = float64(reqCount) / grainSec
			if reqCount > 0 {
				errorPct = (float64(errCount) / float64(reqCount)) * 100.0
			}
		}
		points = append(points, ServicePerformancePoint{
			Timestamp:    t,
			RPS:          rps,
			RequestCount: reqCount,
			ErrorCount:   errCount,
			ErrorPct:     utils.SanitizeFloat(errorPct),
		})
	}
	return points, nil
}

func (s *Service) GetRequestRateTimeSeries(ctx context.Context, f REDFilters) ([]RequestRatePoint, error) {
	rows, err := s.repo.GetRequestRateTimeSeries(ctx, f)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	grainSec := float64(grain.Seconds())
	if grainSec <= 0 {
		grainSec = 60
	}

	startTime := time.UnixMilli(f.StartMs).UTC().Truncate(grain)
	endTime := time.UnixMilli(f.EndMs).UTC().Truncate(grain)

	type key struct {
		serviceName string
		timestamp   int64
	}
	rowMap := make(map[key]uint64)
	servicesSet := make(map[string]struct{})

	for _, row := range rows {
		ts := row.BucketAt.UTC().Truncate(grain).Unix()
		rowMap[key{serviceName: row.ServiceName, timestamp: ts}] = row.RequestCount
		servicesSet[row.ServiceName] = struct{}{}
	}

	var points []RequestRatePoint
	for t := startTime; !t.After(endTime); t = t.Add(grain) {
		ts := t.Unix()
		for svc := range servicesSet {
			reqCount := rowMap[key{serviceName: svc, timestamp: ts}]
			rps := float64(reqCount) / grainSec
			points = append(points, RequestRatePoint{
				Timestamp:   t,
				ServiceName: svc,
				RPS:         utils.SanitizeFloat(rps),
			})
		}
	}
	return points, nil
}

func (s *Service) GetStatusTimeSeries(ctx context.Context, f REDFilters) ([]StatusTimeSeriesPoint, error) {
	rows, err := s.repo.GetStatusTimeSeries(ctx, f)
	if err != nil {
		return nil, err
	}
	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	grainSec := float64(grain.Seconds())
	if grainSec <= 0 {
		grainSec = 60
	}

	startTime := time.UnixMilli(f.StartMs).UTC().Truncate(grain)
	endTime := time.UnixMilli(f.EndMs).UTC().Truncate(grain)

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
	for t := startTime; !t.After(endTime); t = t.Add(grain) {
		pt, ok := byTs[t.Unix()]
		if ok {
			points = append(points, *pt)
		} else {
			points = append(points, StatusTimeSeriesPoint{
				Timestamp: t,
			})
		}
	}
	return points, nil
}

func writeStatusCount(pt *StatusTimeSeriesPoint, bucket string, count float64) {
	switch bucket {
	case "2xx":
		pt.Status2xx += count
	case "4xx":
		pt.Status4xx += count
	case "5xx":
		pt.Status5xx += count
	default:
		pt.StatusOther += count
	}
}

func (s *Service) GetLatencyPercentilesTimeSeries(ctx context.Context, f REDFilters) ([]LatencyPercentilesPoint, error) {
	rows, err := s.repo.GetLatencyPercentilesTimeSeries(ctx, f)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	startTime := time.UnixMilli(f.StartMs).UTC().Truncate(grain)
	endTime := time.UnixMilli(f.EndMs).UTC().Truncate(grain)

	rowMap := make(map[int64]latencyPercentilesTimeseriesRow)
	for _, row := range rows {
		ts := row.BucketAt.UTC().Truncate(grain).Unix()
		rowMap[ts] = row
	}

	var points []LatencyPercentilesPoint
	for t := startTime; !t.After(endTime); t = t.Add(grain) {
		row, ok := rowMap[t.Unix()]
		var p50, p95, p99 float64
		if ok {
			p50 = utils.SanitizeFloat(float64(row.P50Ms))
			p95 = utils.SanitizeFloat(float64(row.P95Ms))
			p99 = utils.SanitizeFloat(float64(row.P99Ms))
		}
		points = append(points, LatencyPercentilesPoint{
			Timestamp: t,
			P50Ms:     p50,
			P95Ms:     p95,
			P99Ms:     p99,
		})
	}
	return points, nil
}

func (s *Service) GetREDByEndpointTimeSeries(ctx context.Context, f REDFilters) ([]EndpointRatePoint, error) {
	rows, err := s.repo.GetREDByEndpointTimeSeries(ctx, f)
	if err != nil {
		return nil, err
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
		var errRate, p99 float64
		if row.RequestCount > 0 {
			errRate = float64(row.ErrorCount) / float64(row.RequestCount)
		}
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

	buckets := denseBuckets(f.StartMs, f.EndMs, grain)
	points := make([]EndpointRatePoint, 0, len(buckets)*len(routes))
	for _, bucket := range buckets {
		for _, route := range routes {
			pt := EndpointRatePoint{Timestamp: bucket, HTTPRoute: route}
			if c, ok := traffic[bucket][route]; ok {
				errRate, p99 := c.errRate, c.p99
				pt.RPS, pt.ErrorRate, pt.P99Ms = c.rps, &errRate, &p99
			}
			points = append(points, pt)
		}
	}
	return points, nil
}

func denseBuckets(startMs, endMs int64, grain time.Duration) []time.Time {
	start := time.UnixMilli(startMs).UTC().Truncate(grain)
	end := time.UnixMilli(endMs).UTC().Truncate(grain)
	var out []time.Time
	for b := start; !b.After(end); b = b.Add(grain) {
		out = append(out, b)
	}
	return out
}

func (s *Service) GetTopEndpointsCombined(
	ctx context.Context, f REDFilters, limit int, cursorIn TopEndpointsCursor,
) (PaginatedEndpoints, error) {
	rows, err := s.repo.GetTopEndpointsCombined(ctx, f, limit+1, cursorIn)
	if err != nil {
		return PaginatedEndpoints{}, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	durationSec := float64(f.EndMs-f.StartMs) / 1000.0
	if durationSec <= 0 {
		durationSec = 1
	}

	results := make([]TopEndpoint, len(rows))
	for i, row := range rows {
		results[i] = toTopEndpoint(row, durationSec)
	}

	var nextCursor string
	if hasMore && len(rows) > 0 {
		lastRow := rows[len(rows)-1]
		nextCursor = cursor.Encode(TopEndpointsCursor{
			TotalCount:    lastRow.TotalCount,
			OperationName: lastRow.OperationName,
		})
	}

	return PaginatedEndpoints{
		Results: results,
		PageInfo: PageInfo{
			HasMore:    hasMore,
			NextCursor: nextCursor,
			Limit:      limit,
		},
	}, nil
}

func (s *Service) GetTopDBQueries(
	ctx context.Context, f REDFilters, limit int, cursorIn TopEndpointsCursor,
) (PaginatedDBQueries, error) {
	rows, err := s.repo.GetTopDBQueriesCombined(ctx, f, limit+1, cursorIn)
	if err != nil {
		return PaginatedDBQueries{}, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	durationSec := float64(f.EndMs-f.StartMs) / 1000.0
	if durationSec <= 0 {
		durationSec = 1
	}

	results := make([]TopDBQuery, len(rows))
	for i, row := range rows {
		results[i] = toTopDBQuery(row, durationSec)
	}

	var nextCursor string
	if hasMore && len(rows) > 0 {
		lastRow := rows[len(rows)-1]
		nextCursor = cursor.Encode(TopEndpointsCursor{
			TotalCount:    lastRow.TotalCount,
			OperationName: lastRow.OperationName,
		})
	}

	return PaginatedDBQueries{
		Results: results,
		PageInfo: PageInfo{
			HasMore:    hasMore,
			NextCursor: nextCursor,
			Limit:      limit,
		},
	}, nil
}

func toTopDBQuery(row topDBQueryRow, durationSec float64) TopDBQuery {
	total := int64(row.TotalCount)
	errs := int64(row.ErrorCount)
	errRate := 0.0
	if total > 0 {
		errRate = float64(errs) / float64(total)
	}
	return TopDBQuery{
		OperationName: row.OperationName,
		ServiceName:   row.ServiceName,
		DBSystem:      row.DBSystem,
		RPS:           float64(total) / durationSec,
		ErrorRate:     errRate,
		ErrorCount:    errs,
		TotalCount:    total,
		P50Ms:         utils.SanitizeFloat(float64(row.P50Ms)),
		P95Ms:         utils.SanitizeFloat(float64(row.P95Ms)),
		P99Ms:         utils.SanitizeFloat(float64(row.P99Ms)),
	}
}

func toTopEndpoint(row topEndpointRow, durationSec float64) TopEndpoint {
	total := int64(row.TotalCount)
	errs := int64(row.ErrorCount)
	errRate := 0.0
	if total > 0 {
		errRate = float64(errs) / float64(total)
	}
	return TopEndpoint{
		OperationName: row.OperationName,
		ServiceName:   row.ServiceName,
		SpanKind:      row.SpanKind,
		HTTPRoute:     row.HTTPRoute,
		RPS:           float64(total) / durationSec,
		ErrorRate:     errRate,
		ErrorCount:    errs,
		TotalCount:    total,
		P50Ms:         utils.SanitizeFloat(float64(row.P50Ms)),
		P95Ms:         utils.SanitizeFloat(float64(row.P95Ms)),
		P99Ms:         utils.SanitizeFloat(float64(row.P99Ms)),
	}
}

func (s *Service) GetOperationBaseline(ctx context.Context, teamID int64, startMs, endMs int64, serviceName, operationName string) (OperationBaseline, error) {
	row, err := s.repo.GetOperationBaseline(ctx, teamID, startMs, endMs, serviceName, operationName)
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
	redRows, err := s.repo.GetFleetREDMetrics(ctx, f)
	if err != nil {
		return ServiceSummaryResponse{}, err
	}
	var redRow *redMetricsRow
	if len(redRows) > 0 {
		redRow = &redRows[0]
	}

	metricNames := []string{
		infraconsts.MetricSystemCPUUtilization,
		infraconsts.MetricSystemCPUUsage,
		infraconsts.MetricProcessCPUUsage,
		infraconsts.MetricJVMCPUUtilization,
		infraconsts.MetricSystemMemoryUtilization,
		infraconsts.MetricSystemDiskUtilization,
	}

	sats, err := s.repo.GetServiceSaturationAggs(ctx, f.TeamID, f.StartMs, f.EndMs, serviceName, metricNames)
	if err != nil {
		sats = nil
	}

	// Map saturation
	var cpuValues []float64
	var memVal float64
	var diskVal float64

	for _, row := range sats {
		switch row.MetricName {
		case infraconsts.MetricSystemCPUUtilization, infraconsts.MetricSystemCPUUsage, infraconsts.MetricProcessCPUUsage, infraconsts.MetricJVMCPUUtilization:
			if v := normalizeUtilization(row.Value); v != nil {
				cpuValues = append(cpuValues, *v)
			}
		case infraconsts.MetricSystemMemoryUtilization:
			if v := normalizeUtilization(row.Value); v != nil {
				memVal = *v
			}
		case infraconsts.MetricSystemDiskUtilization:
			if v := normalizeUtilization(row.Value); v != nil {
				diskVal = *v
			}
		}
	}

	var cpuVal float64
	cpuAvg := averageFloats(cpuValues)
	if cpuAvg != nil {
		cpuVal = *cpuAvg
	}

	var reqCount, errCount int64
	var rps, errRate float64
	var p50, p95, p99 float64

	durationSec := float64(f.EndMs-f.StartMs) / 1000.0
	if durationSec <= 0 {
		durationSec = 1
	}

	if redRow != nil {
		reqCount = int64(redRow.TotalCount)
		errCount = int64(redRow.ErrorCount)
		rps = float64(reqCount) / durationSec
		if reqCount > 0 {
			errRate = float64(errCount) * 100.0 / float64(reqCount)
		}
		p50 = utils.SanitizeFloat(float64(redRow.P50Ms))
		p95 = utils.SanitizeFloat(float64(redRow.P95Ms))
		p99 = utils.SanitizeFloat(float64(redRow.P99Ms))
	}

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

func normalizeUtilization(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > infraconsts.PercentageThreshold*100 {
		return nil
	}
	if v <= infraconsts.PercentageThreshold {
		v = v * infraconsts.PercentageMultiplier
	}
	return &v
}

func averageFloats(vals []float64) *float64 {
	if len(vals) == 0 {
		return nil
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	avg := sum / float64(len(vals))
	return &avg
}

func (s *Service) GetServiceSaturationTimeSeries(ctx context.Context, f REDFilters) ([]SaturationTimeSeriesPoint, error) {
	serviceName := f.SingleService()
	metricNames := []string{
		infraconsts.MetricSystemCPUUtilization,
		infraconsts.MetricSystemCPUUsage,
		infraconsts.MetricProcessCPUUsage,
		infraconsts.MetricJVMCPUUtilization,
	}

	rows, err := s.repo.GetServiceSaturationTimeSeries(ctx, f.TeamID, f.StartMs, f.EndMs, serviceName, metricNames)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	startTime := time.UnixMilli(f.StartMs).UTC().Truncate(grain)
	endTime := time.UnixMilli(f.EndMs).UTC().Truncate(grain)

	rowMap := make(map[int64]saturationTimeSeriesRawRow)
	for _, row := range rows {
		ts := row.BucketAt.UTC().Truncate(grain).Unix()
		rowMap[ts] = row
	}

	var points []SaturationTimeSeriesPoint
	for t := startTime; !t.After(endTime); t = t.Add(grain) {
		row, ok := rowMap[t.Unix()]
		var val float64
		if ok {
			if normalized := normalizeUtilization(row.Value); normalized != nil {
				val = *normalized
			}
		}
		points = append(points, SaturationTimeSeriesPoint{
			Timestamp: t,
			Value:     utils.SanitizeFloat(val),
		})
	}
	return points, nil
}

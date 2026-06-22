package redfleet

import (
	"context"
	"time"

	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/infra/utils"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// GetFleetServices returns one RED row per service across the whole fleet.
func (s *Service) GetFleetServices(ctx context.Context, teamID int64, startMs, endMs int64) ([]ServiceREDMetric, error) {
	rows, err := s.repo.GetFleetREDMetrics(ctx, teamID, startMs, endMs)
	if err != nil {
		return nil, err
	}
	return mapFleetServices(rows), nil
}

// GetFleetTotals returns the fleet-wide RED rollup KPIs.
func (s *Service) GetFleetTotals(ctx context.Context, teamID int64, startMs, endMs int64) (FleetTotals, error) {
	rows, err := s.repo.GetFleetREDMetrics(ctx, teamID, startMs, endMs)
	if err != nil {
		return FleetTotals{}, err
	}
	return computeFleetTotals(mapFleetServices(rows), startMs, endMs), nil
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

func (s *Service) GetApdex(ctx context.Context, teamID int64, startMs, endMs int64, satisfiedMs, toleratingMs float64, serviceName string) ([]ApdexScore, error) {
	var rows []apdexRow
	var err error
	if serviceName != "" {
		rows, err = s.repo.GetApdexByService(ctx, teamID, startMs, endMs, satisfiedMs, toleratingMs, serviceName)
	} else {
		rows, err = s.repo.GetApdex(ctx, teamID, startMs, endMs, satisfiedMs, toleratingMs)
	}
	if err != nil {
		return nil, err
	}

	result := make([]ApdexScore, len(rows))
	for i, row := range rows {
		total := int64(row.TotalCount)
		satisfied := int64(row.Satisfied)
		tolerating := int64(row.Tolerating)
		frustrated := total - satisfied - tolerating
		if frustrated < 0 {
			frustrated = 0
		}
		apdex := 0.0
		if total > 0 {
			apdex = (float64(satisfied) + float64(tolerating)*0.5) / float64(total)
		}
		result[i] = ApdexScore{
			ServiceName: row.ServiceName,
			Apdex:       apdex,
			Satisfied:   satisfied,
			Tolerating:  tolerating,
			Frustrated:  frustrated,
			TotalCount:  total,
		}
	}
	return result, nil
}

func (s *Service) GetRequestAndErrorRateTimeSeries(ctx context.Context, teamID int64, startMs, endMs int64) ([]ServicePerformancePoint, error) {
	rows, err := s.repo.GetRequestAndErrorRateTimeSeries(ctx, teamID, startMs, endMs)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(endMs - startMs)
	grainSec := float64(grain.Seconds())
	if grainSec <= 0 {
		grainSec = 60
	}

	startTime := time.UnixMilli(startMs).UTC().Truncate(grain)
	endTime := time.UnixMilli(endMs).UTC().Truncate(grain)

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

// GetStatusTimeSeries pivots per-bucket / per-status-class rows into one
// point per bucket with 2xx / 4xx / 5xx (and "other") counts.
func (s *Service) GetStatusTimeSeries(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string,
) ([]StatusTimeSeriesPoint, error) {
	rows, err := s.repo.GetStatusTimeSeries(ctx, teamID, startMs, endMs, serviceName)
	if err != nil {
		return nil, err
	}
	grain := timebucket.DisplayGrain(endMs - startMs)
	grainSec := float64(grain.Seconds())
	if grainSec <= 0 {
		grainSec = 60
	}

	startTime := time.UnixMilli(startMs).UTC().Truncate(grain)
	endTime := time.UnixMilli(endMs).UTC().Truncate(grain)

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

// GetLatencyPercentilesTimeSeries returns p50/p95/p99 over time for one service
// (or for all team services when serviceName is empty).
func (s *Service) GetLatencyPercentilesTimeSeries(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string,
) ([]LatencyPercentilesPoint, error) {
	rows, err := s.repo.GetLatencyPercentilesTimeSeries(ctx, teamID, startMs, endMs, serviceName)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(endMs - startMs)
	startTime := time.UnixMilli(startMs).UTC().Truncate(grain)
	endTime := time.UnixMilli(endMs).UTC().Truncate(grain)

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

// GetREDByEndpointTimeSeries returns per-(bucket, route) rps / error rate / p99
// for the per-endpoint golden-signal lines. Rows are emitted only for buckets
// that carry traffic; the frontend aligns them onto a shared time axis.
func (s *Service) GetREDByEndpointTimeSeries(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string,
) ([]EndpointRatePoint, error) {
	rows, err := s.repo.GetREDByEndpointTimeSeries(ctx, teamID, startMs, endMs, serviceName)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(endMs - startMs)
	grainSec := float64(grain.Seconds())
	if grainSec <= 0 {
		grainSec = 60
	}

	// Index traffic rows by (bucket, route); routes preserve first-seen order.
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

	// Emit a point for every (bucket, route) across the dense window axis so
	// quiet buckets render as 0 rps (line breaks for latency/error) instead of
	// collapsing the axis to a straight 2-point line.
	buckets := denseBuckets(startMs, endMs, grain)
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

// denseBuckets returns every display-grain bucket in [start, end], aligned to
// the same truncation the rollup query uses, so gaps become explicit points.
func denseBuckets(startMs, endMs int64, grain time.Duration) []time.Time {
	start := time.UnixMilli(startMs).UTC().Truncate(grain)
	end := time.UnixMilli(endMs).UTC().Truncate(grain)
	var out []time.Time
	for b := start; !b.After(end); b = b.Add(grain) {
		out = append(out, b)
	}
	return out
}

// GetTopEndpointsCombined returns per-operation rate / errPct / p50 / p95 / p99
// sorted by request volume.
func (s *Service) GetTopEndpointsCombined(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string, limit int, cursorIn TopEndpointsCursor,
) (PaginatedEndpoints, error) {
	rows, err := s.repo.GetTopEndpointsCombined(ctx, teamID, startMs, endMs, serviceName, limit+1, cursorIn)
	if err != nil {
		return PaginatedEndpoints{}, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	durationSec := float64(endMs-startMs) / 1000.0
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

// GetTopDBQueries returns per-query rate / errPct / p50 / p95 / p99 for the
// service's database calls, sorted by request volume.
func (s *Service) GetTopDBQueries(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string, limit int, cursorIn TopEndpointsCursor,
) (PaginatedDBQueries, error) {
	rows, err := s.repo.GetTopDBQueriesCombined(ctx, teamID, startMs, endMs, serviceName, limit+1, cursorIn)
	if err != nil {
		return PaginatedDBQueries{}, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	durationSec := float64(endMs-startMs) / 1000.0
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

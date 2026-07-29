package redfleet

import (
	"context"
	"sort"
	"time"

	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/httputil"
	"github.com/optikklabs/query/internal/shared/metrics"
	"github.com/optikklabs/query/internal/shared/spanstats"
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
				pt.ErrorRate = httputil.SanitizeFloat(metrics.Percentage(row.ErrorCount, row.RequestCount))
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

	services, buckets, series := timebucket.FillGapsKeyed(f.StartMs, f.EndMs, grain, rows,
		func(r serviceRequestRateRawRow) string { return r.ServiceName },
		func(r serviceRequestRateRawRow) time.Time { return r.BucketAt },
		func(_ time.Time, row serviceRequestRateRawRow, _ bool) float64 {
			return httputil.SanitizeFloat(float64(row.RequestCount) / grainSec)
		})

	out := RequestRateSeries{
		Timestamps: make([]int64, len(buckets)),
		Series:     make([]RequestRateEntry, len(services)),
	}
	for i, t := range buckets {
		out.Timestamps[i] = t.UnixMilli()
	}
	for i, svc := range services {
		out.Series[i] = RequestRateEntry{ServiceName: svc, RPS: series[i]}
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

	return timebucket.FillGaps(f.StartMs, f.EndMs, grain, rows,
		func(r statusBucketTimeseriesRow) time.Time { return r.BucketAt },
		func(t time.Time, row statusBucketTimeseriesRow, ok bool) StatusTimeSeriesPoint {
			pt := StatusTimeSeriesPoint{Timestamp: t}
			if ok {
				pt.Status2xx = float64(row.Status2xx) / grainSec
				pt.Status4xx = float64(row.Status4xx) / grainSec
				pt.Status5xx = float64(row.Status5xx) / grainSec
				pt.StatusOther = float64(row.StatusOther) / grainSec
			}
			return pt
		}), nil
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
				pt.P50Ms = httputil.SanitizeFloat(float64(row.P50Ms))
				pt.P95Ms = httputil.SanitizeFloat(float64(row.P95Ms))
				pt.P99Ms = httputil.SanitizeFloat(float64(row.P99Ms))
			}
			return pt
		}), nil
}

type serviceBucketTotal struct {
	BucketAt     time.Time
	RequestCount uint64
	ErrorCount   uint64
}

func (s *Service) GetREDByEndpointTimeSeries(ctx context.Context, f REDFilters, limit int) (EndpointRateSeries, error) {
	rows, err := s.repo.GetREDByEndpointTimeSeries(ctx, f)
	if err != nil {
		return EndpointRateSeries{}, err
	}
	if limit <= 0 {
		limit = defaultEndpointSeriesLimit
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	grainSec := float64(grain.Seconds())
	if grainSec <= 0 {
		grainSec = 60
	}

	// Totals cover the whole service, so they are summed before the top-N cut.
	totalsByBucket := make(map[int64]*serviceBucketTotal, len(rows))
	for _, row := range rows {
		key := row.BucketAt.UTC().Truncate(grain).Unix()
		acc, ok := totalsByBucket[key]
		if !ok {
			acc = &serviceBucketTotal{BucketAt: row.BucketAt}
			totalsByBucket[key] = acc
		}
		acc.RequestCount += row.RequestCount
		acc.ErrorCount += row.ErrorCount
	}
	totalRows := make([]serviceBucketTotal, 0, len(totalsByBucket))
	for _, acc := range totalsByBucket {
		totalRows = append(totalRows, *acc)
	}

	type cell struct {
		rps     float64
		count   uint64
		errRate *float64
		p99     *float64
	}

	charted := topEndpointRows(rows, limit)

	operations, buckets, series := timebucket.FillGapsKeyed(f.StartMs, f.EndMs, grain, charted,
		func(r endpointRateRow) string { return r.OperationName },
		func(r endpointRateRow) time.Time { return r.BucketAt },
		func(_ time.Time, row endpointRateRow, ok bool) cell {
			if !ok {
				return cell{}
			}
			errRate := metrics.Percentage(row.ErrorCount, row.RequestCount)
			p99 := httputil.SanitizeFloat(spanstats.LatencyP99.At(row.QS, spanstats.P99))
			return cell{
				rps:     float64(row.RequestCount) / grainSec,
				count:   row.RequestCount,
				errRate: &errRate,
				p99:     &p99,
			}
		})

	totals := timebucket.FillGaps(f.StartMs, f.EndMs, grain, totalRows,
		func(r serviceBucketTotal) time.Time { return r.BucketAt },
		func(_ time.Time, row serviceBucketTotal, ok bool) cell {
			if !ok {
				return cell{}
			}
			errRate := metrics.Percentage(row.ErrorCount, row.RequestCount)
			return cell{
				rps:     float64(row.RequestCount) / grainSec,
				count:   row.RequestCount,
				errRate: &errRate,
			}
		})

	out := EndpointRateSeries{
		Timestamps: make([]int64, len(buckets)),
		Series:     make([]EndpointRateEntry, len(operations)),
		Totals: EndpointRateTotals{
			RPS:          make([]float64, len(buckets)),
			RequestCount: make([]uint64, len(buckets)),
			ErrorRate:    make([]*float64, len(buckets)),
		},
	}
	for i, bucket := range buckets {
		out.Timestamps[i] = bucket.UnixMilli()
	}
	for i, c := range totals {
		out.Totals.RPS[i], out.Totals.RequestCount[i], out.Totals.ErrorRate[i] = c.rps, c.count, c.errRate
	}
	for i, operation := range operations {
		entry := EndpointRateEntry{
			OperationName: operation,
			RPS:           make([]float64, len(buckets)),
			RequestCount:  make([]uint64, len(buckets)),
			ErrorRate:     make([]*float64, len(buckets)),
			P99Ms:         make([]*float64, len(buckets)),
		}
		for j, c := range series[i] {
			entry.RPS[j], entry.RequestCount[j] = c.rps, c.count
			entry.ErrorRate[j], entry.P99Ms[j] = c.errRate, c.p99
		}
		out.Series[i] = entry
	}
	// FillGapsKeyed returns endpoints in first-encounter order, which is the
	// arbitrary order of the earliest bucket. Rank by volume so the chart
	// legend reads busiest-first and colors stay stable across refreshes.
	sort.SliceStable(out.Series, func(i, j int) bool {
		li, lj := seriesLoad(out.Series[i].RPS), seriesLoad(out.Series[j].RPS)
		if li != lj {
			return li > lj
		}
		return out.Series[i].OperationName < out.Series[j].OperationName
	})
	return out, nil
}

// topEndpointRows keeps the rows of the busiest `limit` named endpoints.
func topEndpointRows(rows []endpointRateRow, limit int) []endpointRateRow {
	volume := make(map[string]uint64)
	for _, row := range rows {
		if row.OperationName != "" {
			volume[row.OperationName] += row.RequestCount
		}
	}
	names := make([]string, 0, len(volume))
	for name := range volume {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if volume[names[i]] != volume[names[j]] {
			return volume[names[i]] > volume[names[j]]
		}
		return names[i] < names[j]
	})
	if len(names) > limit {
		names = names[:limit]
	}

	keep := make(map[string]struct{}, len(names))
	for _, name := range names {
		keep[name] = struct{}{}
	}
	out := make([]endpointRateRow, 0, len(rows))
	for _, row := range rows {
		if _, ok := keep[row.OperationName]; ok {
			out = append(out, row)
		}
	}
	return out
}

func seriesLoad(rps []float64) float64 {
	var total float64
	for _, v := range rps {
		total += v
	}
	return total
}

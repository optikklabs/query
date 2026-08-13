package service

import (
	"context"
	"sort"
	"time"

	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/services/redfleet/filter"
	"github.com/optikklabs/query/internal/modules/services/redfleet/models"
	"github.com/optikklabs/query/internal/modules/services/redfleet/repository"
	"github.com/optikklabs/query/internal/shared/httputil"
	"github.com/optikklabs/query/internal/shared/metrics"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

// Series count when the caller does not ask for one; handler caps at MaxPageSize.
const defaultEndpointSeriesLimit = 20

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetFleetOverview(ctx context.Context, f filter.Filters) (models.FleetOverviewResponse, error) {
	rows, err := s.repo.GetFleetREDMetrics(ctx, f)
	if err != nil {
		return models.FleetOverviewResponse{}, err
	}
	services := mapFleetServices(rows)
	return models.FleetOverviewResponse{
		Totals:   computeFleetTotals(fleetTotalRow(rows), len(services), f.StartMs, f.EndMs),
		Services: services,
	}, nil
}

func (s *Service) GetRequestAndErrorRateTimeSeries(ctx context.Context, f filter.Filters) ([]models.ServicePerformancePoint, error) {
	rows, err := s.repo.GetRequestAndErrorRateTimeSeries(ctx, f)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	grainSec := grain.Seconds()
	return timebucket.FillGaps(f.StartMs, f.EndMs, grain, rows,
		func(r models.RequestRateRawRow) time.Time { return r.BucketAt },
		func(t time.Time, row models.RequestRateRawRow, ok bool) models.ServicePerformancePoint {
			pt := models.ServicePerformancePoint{TimestampMs: t.UnixMilli()}
			if ok {
				pt.RequestCount = row.RequestCount
				pt.ErrorCount = row.ErrorCount
				pt.RPS = float64(row.RequestCount) / grainSec
				pt.ErrorRate = httputil.SanitizeFloat(metrics.Percentage(row.ErrorCount, row.RequestCount))
			}
			return pt
		}), nil
}

func (s *Service) GetRequestRateTimeSeries(ctx context.Context, f filter.Filters) (models.RequestRateSeries, error) {
	rows, err := s.repo.GetRequestRateTimeSeries(ctx, f)
	if err != nil {
		return models.RequestRateSeries{}, err
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	grainSec := grain.Seconds()

	services, buckets, series := timebucket.FillGapsKeyed(f.StartMs, f.EndMs, grain, rows,
		func(r models.ServiceRequestRateRow) string { return r.ServiceName },
		func(r models.ServiceRequestRateRow) time.Time { return r.BucketAt },
		func(_ time.Time, row models.ServiceRequestRateRow, _ bool) float64 {
			return httputil.SanitizeFloat(float64(row.RequestCount) / grainSec)
		})

	out := models.RequestRateSeries{
		Timestamps: make([]int64, len(buckets)),
		Series:     make([]models.RequestRateEntry, len(services)),
	}
	for i, t := range buckets {
		out.Timestamps[i] = t.UnixMilli()
	}
	for i, svc := range services {
		out.Series[i] = models.RequestRateEntry{ServiceName: svc, RPS: series[i]}
	}
	return out, nil
}

func (s *Service) GetStatusTimeSeries(ctx context.Context, f filter.Filters) ([]models.StatusTimeSeriesPoint, error) {
	rows, err := s.repo.GetStatusTimeSeries(ctx, f)
	if err != nil {
		return nil, err
	}
	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	grainSec := grain.Seconds()

	return timebucket.FillGaps(f.StartMs, f.EndMs, grain, rows,
		func(r models.StatusBucketRow) time.Time { return r.BucketAt },
		func(t time.Time, row models.StatusBucketRow, ok bool) models.StatusTimeSeriesPoint {
			pt := models.StatusTimeSeriesPoint{TimestampMs: t.UnixMilli()}
			if ok {
				pt.Status2xx = float64(row.Status2xx) / grainSec
				pt.Status4xx = float64(row.Status4xx) / grainSec
				pt.Status5xx = float64(row.Status5xx) / grainSec
				pt.StatusOther = float64(row.StatusOther) / grainSec
			}
			return pt
		}), nil
}

func (s *Service) GetLatencyPercentilesTimeSeries(ctx context.Context, f filter.Filters) ([]models.LatencyPercentilesPoint, error) {
	rows, err := s.repo.GetLatencyPercentilesTimeSeries(ctx, f)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	return timebucket.FillGaps(f.StartMs, f.EndMs, grain, rows,
		func(r models.LatencyPercentilesRow) time.Time { return r.BucketAt },
		func(t time.Time, row models.LatencyPercentilesRow, ok bool) models.LatencyPercentilesPoint {
			pt := models.LatencyPercentilesPoint{TimestampMs: t.UnixMilli()}
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

type endpointSeriesCell struct {
	rps     float64
	count   uint64
	errRate *float64
	p99     *float64
}

func (s *Service) GetREDByEndpointTimeSeries(ctx context.Context, f filter.Filters, limit int) (models.EndpointRateSeries, error) {
	rows, err := s.repo.GetREDByEndpointTimeSeries(ctx, f)
	if err != nil {
		return models.EndpointRateSeries{}, err
	}
	return buildEndpointRateSeries(rows, f, limit), nil
}

func buildEndpointRateSeries(rows []models.EndpointRateRow, f filter.Filters, limit int) models.EndpointRateSeries {
	if limit <= 0 {
		limit = defaultEndpointSeriesLimit
	}
	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	grainSec := float64(grain.Seconds())
	if grainSec <= 0 {
		grainSec = 60
	}
	operations, buckets, series := timebucket.FillGapsKeyed(f.StartMs, f.EndMs, grain, topEndpointRows(rows, limit),
		func(r models.EndpointRateRow) string { return r.OperationName },
		func(r models.EndpointRateRow) time.Time { return r.BucketAt },
		endpointCell(grainSec))
	totals := timebucket.FillGaps(f.StartMs, f.EndMs, grain, serviceTotalRows(rows, grain),
		func(r serviceBucketTotal) time.Time { return r.BucketAt },
		totalCell(grainSec))
	return assembleEndpointRateSeries(operations, buckets, series, totals)
}

func serviceTotalRows(rows []models.EndpointRateRow, grain time.Duration) []serviceBucketTotal {
	byBucket := make(map[int64]*serviceBucketTotal, len(rows))
	for _, row := range rows {
		key := row.BucketAt.UTC().Truncate(grain).Unix()
		if byBucket[key] == nil {
			byBucket[key] = &serviceBucketTotal{BucketAt: row.BucketAt}
		}
		byBucket[key].RequestCount += row.RequestCount
		byBucket[key].ErrorCount += row.ErrorCount
	}
	totals := make([]serviceBucketTotal, 0, len(byBucket))
	for _, total := range byBucket {
		totals = append(totals, *total)
	}
	return totals
}

func endpointCell(grainSec float64) func(time.Time, models.EndpointRateRow, bool) endpointSeriesCell {
	return func(_ time.Time, row models.EndpointRateRow, ok bool) endpointSeriesCell {
		if !ok {
			return endpointSeriesCell{}
		}
		errRate := metrics.Percentage(row.ErrorCount, row.RequestCount)
		var p99 float64
		if len(row.QS) > 0 {
			p99 = httputil.SanitizeFloat(spanstats.LatencyP99.At(row.QS, spanstats.P99))
		}
		return endpointSeriesCell{float64(row.RequestCount) / grainSec, row.RequestCount, &errRate, &p99}
	}
}

func totalCell(grainSec float64) func(time.Time, serviceBucketTotal, bool) endpointSeriesCell {
	return func(_ time.Time, row serviceBucketTotal, ok bool) endpointSeriesCell {
		if !ok {
			return endpointSeriesCell{}
		}
		errRate := metrics.Percentage(row.ErrorCount, row.RequestCount)
		return endpointSeriesCell{rps: float64(row.RequestCount) / grainSec, count: row.RequestCount, errRate: &errRate}
	}
}

func assembleEndpointRateSeries(operations []string, buckets []time.Time, series [][]endpointSeriesCell, totals []endpointSeriesCell) models.EndpointRateSeries {
	out := models.EndpointRateSeries{
		Timestamps: make([]int64, len(buckets)),
		Series:     make([]models.EndpointRateEntry, len(operations)),
		Totals: models.EndpointRateTotals{
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
		entry := models.EndpointRateEntry{
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
	sort.SliceStable(out.Series, func(i, j int) bool {
		li, lj := seriesLoad(out.Series[i].RPS), seriesLoad(out.Series[j].RPS)
		if li != lj {
			return li > lj
		}
		return out.Series[i].OperationName < out.Series[j].OperationName
	})
	return out
}

func topEndpointRows(rows []models.EndpointRateRow, limit int) []models.EndpointRateRow {
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
	out := make([]models.EndpointRateRow, 0, len(rows))
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

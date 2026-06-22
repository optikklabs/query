package redservice

import (
	"context"
	"math"
	"time"

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

func (s *Service) GetServiceSummary(ctx context.Context, teamID int64, startMs, endMs int64, serviceName string) (ServiceSummaryResponse, error) {
	redRow, err := s.repo.GetServiceREDMetrics(ctx, teamID, startMs, endMs, serviceName)
	if err != nil {
		return ServiceSummaryResponse{}, err
	}

	metricNames := []string{
		infraconsts.MetricSystemCPUUtilization,
		infraconsts.MetricSystemCPUUsage,
		infraconsts.MetricProcessCPUUsage,
		infraconsts.MetricJVMCPUUtilization,
		infraconsts.MetricSystemMemoryUtilization,
		infraconsts.MetricSystemDiskUtilization,
	}

	sats, err := s.repo.GetServiceSaturationAggs(ctx, teamID, startMs, endMs, serviceName, metricNames)
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

	durationSec := float64(endMs-startMs) / 1000.0
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

func (s *Service) GetServiceSaturationTimeSeries(
	ctx context.Context, teamID int64, startMs, endMs int64, serviceName string,
) ([]SaturationTimeSeriesPoint, error) {
	metricNames := []string{
		infraconsts.MetricSystemCPUUtilization,
		infraconsts.MetricSystemCPUUsage,
		infraconsts.MetricProcessCPUUsage,
		infraconsts.MetricJVMCPUUtilization,
	}

	rows, err := s.repo.GetServiceSaturationTimeSeries(ctx, teamID, startMs, endMs, serviceName, metricNames)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(endMs - startMs)
	startTime := time.UnixMilli(startMs).UTC().Truncate(grain)
	endTime := time.UnixMilli(endMs).UTC().Truncate(grain)

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

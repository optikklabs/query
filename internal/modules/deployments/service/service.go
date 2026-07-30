package service

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/deployments/models"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
	"github.com/optikklabs/query/internal/shared/metrics"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

type Repository interface {
	ListRows(context.Context, models.ListRequest) ([]models.RawDeploymentRow, error)
	ComparisonRow(context.Context, models.DetailRequest, models.Context) (models.RawComparisonRow, error)
	TrafficRows(context.Context, models.DetailRequest, models.Context) ([]models.RawTrafficRow, error)
	ErrorChangeRows(context.Context, models.DetailRequest, models.Context) ([]models.RawErrorChangeRow, error)
	DimensionDiffRows(context.Context, models.DetailRequest, models.Context, string) ([]models.RawDimensionDiffRow, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, req models.ListRequest) (models.ListResponse, error) {
	rows, err := s.repo.ListRows(ctx, req)
	if err != nil {
		return models.ListResponse{}, err
	}
	return buildListResponse(rows, req.EndMs), nil
}

func (s *Service) Compare(ctx context.Context, req models.DetailRequest) (models.CompareResponse, error) {
	comparison, err := s.resolveContext(ctx, req)
	if err != nil {
		return models.CompareResponse{}, err
	}
	row, err := s.repo.ComparisonRow(ctx, req, comparison)
	if err != nil {
		return models.CompareResponse{}, err
	}
	return models.CompareResponse{
		Context: comparison,
		Metrics: comparisonMetrics(row, comparison.BaselineVersion != nil),
	}, nil
}

func (s *Service) Traffic(ctx context.Context, req models.DetailRequest) (models.TrafficResponse, error) {
	comparison, err := s.resolveContext(ctx, req)
	if err != nil {
		return models.TrafficResponse{}, err
	}
	rows, err := s.repo.TrafficRows(ctx, req, comparison)
	if err != nil {
		return models.TrafficResponse{}, err
	}

	grain := timebucket.DisplayGrainForRange(
		comparison.Window.BaselineStart.UnixMilli(),
		comparison.Window.CurrentEnd.UnixMilli(),
	)
	versions, buckets, series := timebucket.FillGapsKeyed(
		comparison.Window.BaselineStart.UnixMilli(),
		comparison.Window.CurrentEnd.UnixMilli(),
		grain,
		rows,
		func(row models.RawTrafficRow) string { return row.Version },
		func(row models.RawTrafficRow) time.Time { return row.BucketAt },
		func(_ time.Time, row models.RawTrafficRow, ok bool) uint64 {
			if !ok {
				return 0
			}
			return row.Requests
		},
	)
	out := models.TrafficResponse{
		Context:    comparison,
		Timestamps: make([]int64, len(buckets)),
		Series:     make([]models.TrafficSeries, len(versions)),
	}
	for i, bucket := range buckets {
		out.Timestamps[i] = bucket.UnixMilli()
	}
	for i, version := range versions {
		out.Series[i] = models.TrafficSeries{Version: version, Requests: series[i]}
	}
	sort.SliceStable(out.Series, func(i, j int) bool {
		if out.Series[i].Version == req.Version {
			return true
		}
		if out.Series[j].Version == req.Version {
			return false
		}
		return out.Series[i].Version < out.Series[j].Version
	})
	return out, nil
}

func (s *Service) Errors(ctx context.Context, req models.DetailRequest) (models.ErrorChangesResponse, error) {
	comparison, err := s.resolveContext(ctx, req)
	if err != nil {
		return models.ErrorChangesResponse{}, err
	}
	if comparison.BaselineVersion == nil {
		return models.ErrorChangesResponse{
			Context:  comparison,
			New:      []models.ErrorChange{},
			Resolved: []models.ErrorChange{},
		}, nil
	}
	rows, err := s.repo.ErrorChangeRows(ctx, req, comparison)
	if err != nil {
		return models.ErrorChangesResponse{}, err
	}
	out := models.ErrorChangesResponse{
		Context:  comparison,
		New:      make([]models.ErrorChange, 0),
		Resolved: make([]models.ErrorChange, 0),
	}
	for _, row := range rows {
		change := models.ErrorChange{
			GroupID:       row.GroupID,
			OperationName: row.OperationName,
			ExceptionType: row.ExceptionType,
			CurrentCount:  row.CurrentCount,
			BaselineCount: row.BaselineCount,
		}
		switch {
		case row.CurrentCount > 0 && row.BaselineCount == 0:
			out.New = append(out.New, change)
		case row.CurrentCount == 0 && row.BaselineCount > 0:
			out.Resolved = append(out.Resolved, change)
		}
	}
	return out, nil
}

func (s *Service) Endpoints(ctx context.Context, req models.DetailRequest) (models.DimensionDiffResponse, error) {
	return s.dimensionDiff(ctx, req, "endpoints")
}

func (s *Service) Dependencies(ctx context.Context, req models.DetailRequest) (models.DimensionDiffResponse, error) {
	return s.dimensionDiff(ctx, req, "dependencies")
}

func (s *Service) dimensionDiff(
	ctx context.Context,
	req models.DetailRequest,
	dimension string,
) (models.DimensionDiffResponse, error) {
	comparison, err := s.resolveContext(ctx, req)
	if err != nil {
		return models.DimensionDiffResponse{}, err
	}
	rows, err := s.repo.DimensionDiffRows(ctx, req, comparison, dimension)
	if err != nil {
		return models.DimensionDiffResponse{}, err
	}
	results := make([]models.DimensionDiff, len(rows))
	hasBaseline := comparison.BaselineVersion != nil
	for i, row := range rows {
		current := redValues(row.CurrentRequests, row.CurrentErrors, row.CurrentQS)
		results[i] = models.DimensionDiff{Name: row.Name, Current: current}
		if hasBaseline {
			baseline := redValues(row.BaselineRequests, row.BaselineErrors, row.BaselineQS)
			results[i].Baseline = &baseline
			results[i].RequestDelta = pointer(float64(current.Requests) - float64(baseline.Requests))
			results[i].ErrorRateDelta = pointer(current.ErrorRate - baseline.ErrorRate)
			results[i].P95DeltaMs = pointer(current.P95Ms - baseline.P95Ms)
		}
	}
	return models.DimensionDiffResponse{Context: comparison, Results: results}, nil
}

func (s *Service) resolveContext(ctx context.Context, req models.DetailRequest) (models.Context, error) {
	if strings.TrimSpace(req.Service) == "" || strings.TrimSpace(req.Version) == "" {
		return models.Context{}, errorcode.ValidationError{Msg: "service and version are required"}
	}
	if !req.EnvironmentSet {
		return models.Context{}, errorcode.ValidationError{Msg: "environment is required"}
	}
	rows, err := s.repo.ListRows(ctx, req.ListRequest)
	if err != nil {
		return models.Context{}, err
	}
	return findContext(rows, req)
}

func findContext(rows []models.RawDeploymentRow, req models.DetailRequest) (models.Context, error) {
	candidates := make([]models.RawDeploymentRow, 0)
	for _, row := range rows {
		if row.Service == req.Service && row.Environment == req.Environment {
			candidates = append(candidates, row)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].FirstSeen.Equal(candidates[j].FirstSeen) {
			return candidates[i].Version < candidates[j].Version
		}
		return candidates[i].FirstSeen.Before(candidates[j].FirstSeen)
	})

	target := -1
	for i := range candidates {
		if candidates[i].Version == req.Version {
			target = i
			break
		}
	}
	if target < 0 {
		return models.Context{}, errorcode.NotFoundError{Msg: "deployment not found in the selected range"}
	}

	row := candidates[target]
	windowEnd := time.UnixMilli(req.EndMs)
	if target+1 < len(candidates) && candidates[target+1].FirstSeen.Before(windowEnd) {
		windowEnd = candidates[target+1].FirstSeen
	}
	windowDuration := windowEnd.Sub(row.FirstSeen)
	if windowDuration <= 0 {
		return models.Context{}, errorcode.ValidationError{Msg: "deployment has no comparable traffic window"}
	}

	out := models.Context{
		Service:     req.Service,
		Environment: req.Environment,
		Version:     req.Version,
		FirstSeen:   row.FirstSeen,
		Window: models.Window{
			CurrentStart:  row.FirstSeen,
			CurrentEnd:    windowEnd,
			BaselineStart: row.FirstSeen.Add(-windowDuration),
			BaselineEnd:   row.FirstSeen,
		},
	}
	if target > 0 {
		out.BaselineVersion = pointer(candidates[target-1].Version)
	}
	return out, nil
}

func buildListResponse(rows []models.RawDeploymentRow, endMs int64) models.ListResponse {
	grouped := make(map[string][]models.RawDeploymentRow)
	environmentSet := make(map[string]struct{})
	serviceSet := make(map[string]struct{})
	for _, row := range rows {
		key := row.Service + "\x00" + row.Environment
		grouped[key] = append(grouped[key], row)
		environmentSet[row.Environment] = struct{}{}
		serviceSet[row.Service] = struct{}{}
	}

	results := make([]models.Deployment, 0, len(rows))
	var latest *time.Time
	for _, group := range grouped {
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].FirstSeen.Equal(group[j].FirstSeen) {
				return group[i].Version < group[j].Version
			}
			return group[i].FirstSeen.Before(group[j].FirstSeen)
		})
		var totalRequests uint64
		for _, row := range group {
			totalRequests += row.Requests
		}
		for i, row := range group {
			p95 := p95From(row.QS)
			item := models.Deployment{
				Service:      row.Service,
				Environment:  row.Environment,
				Version:      row.Version,
				FirstSeen:    row.FirstSeen,
				TimelineEnd:  time.UnixMilli(endMs),
				TrafficShare: metrics.Percentage(row.Requests, totalRequests),
				RequestCount: row.Requests,
				ErrorRate:    metrics.Percentage(row.Errors, row.Requests),
				P95Ms:        p95,
			}
			if i+1 < len(group) {
				item.TimelineEnd = group[i+1].FirstSeen
			}
			if i > 0 {
				previous := group[i-1]
				previousVersion := previous.Version
				previousErrorRate := metrics.Percentage(previous.Errors, previous.Requests)
				previousP95 := p95From(previous.QS)
				item.PreviousVersion = &previousVersion
				item.ErrorRateDelta = pointer(item.ErrorRate - previousErrorRate)
				item.P95DeltaMs = pointer(item.P95Ms - previousP95)
			}
			results = append(results, item)
			if latest == nil || row.FirstSeen.After(*latest) {
				value := row.FirstSeen
				latest = &value
			}
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if !results[i].FirstSeen.Equal(results[j].FirstSeen) {
			return results[i].FirstSeen.After(results[j].FirstSeen)
		}
		if results[i].Service != results[j].Service {
			return results[i].Service < results[j].Service
		}
		if results[i].Environment != results[j].Environment {
			return results[i].Environment < results[j].Environment
		}
		return results[i].Version < results[j].Version
	})

	environments := make([]string, 0, len(environmentSet))
	for environment := range environmentSet {
		environments = append(environments, environment)
	}
	sort.Strings(environments)
	return models.ListResponse{
		Results:      results,
		Environments: environments,
		Summary: models.ListSummary{
			DeploymentCount:  len(results),
			ServiceCount:     len(serviceSet),
			EnvironmentCount: len(environments),
			LatestFirstSeen:  latest,
		},
	}
}

func comparisonMetrics(row models.RawComparisonRow, hasBaseline bool) models.ComparisonMetrics {
	currentLatency := quantiles(row.CurrentQS)
	baselineLatency := quantiles(row.BaselineQS)
	return models.ComparisonMetrics{
		Requests:  compare(float64(row.CurrentRequests), float64(row.BaselineRequests), hasBaseline),
		Errors:    compare(float64(row.CurrentErrors), float64(row.BaselineErrors), hasBaseline),
		ErrorRate: compare(metrics.Percentage(row.CurrentErrors, row.CurrentRequests), metrics.Percentage(row.BaselineErrors, row.BaselineRequests), hasBaseline),
		P50Ms:     compare(currentLatency[0], baselineLatency[0], hasBaseline),
		P75Ms:     compare(currentLatency[1], baselineLatency[1], hasBaseline),
		P90Ms:     compare(currentLatency[2], baselineLatency[2], hasBaseline),
		P95Ms:     compare(currentLatency[3], baselineLatency[3], hasBaseline),
		P99Ms:     compare(currentLatency[4], baselineLatency[4], hasBaseline),
	}
}

func compare(current, baseline float64, hasBaseline bool) models.MetricComparison {
	out := models.MetricComparison{Current: httputil.SanitizeFloat(current)}
	if !hasBaseline {
		return out
	}
	baseline = httputil.SanitizeFloat(baseline)
	delta := httputil.SanitizeFloat(out.Current - baseline)
	out.Baseline = pointer(baseline)
	out.Delta = pointer(delta)
	if baseline != 0 {
		out.DeltaPercent = pointer(httputil.SanitizeFloat(delta / baseline * 100))
	}
	return out
}

func redValues(requests, errors uint64, qs []float64) models.REDValues {
	return models.REDValues{
		Requests:  requests,
		Errors:    errors,
		ErrorRate: metrics.Percentage(errors, requests),
		P95Ms:     p95From(qs),
	}
}

func p95From(qs []float64) float64 {
	if len(qs) == 0 {
		return 0
	}
	return sanitizeQuantile(spanstats.LatencyP95.At(qs, spanstats.P95))
}

func quantiles(qs []float64) [5]float64 {
	var out [5]float64
	if len(qs) < len(out) {
		return out
	}
	for i := range out {
		out[i] = sanitizeQuantile(qs[i])
	}
	return out
}

func sanitizeQuantile(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

func pointer[T any](value T) *T {
	return &value
}

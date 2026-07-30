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

// ---------------------------------------------------------------------------
// Repository contract
// ---------------------------------------------------------------------------

type Repository interface {
	ListRows(context.Context, models.ListRequest) ([]models.RawDeploymentRow, error)
	ComparisonRow(context.Context, models.DetailRequest, models.Context) (models.RawComparisonRow, error)
	TrafficRows(context.Context, models.DetailRequest, models.Context) ([]models.RawTrafficRow, error)
	ErrorChangeRows(context.Context, models.DetailRequest, models.Context) ([]models.RawErrorChangeRow, error)
	DimensionDiffRows(context.Context, models.DetailRequest, models.Context, string) ([]models.RawDimensionDiffRow, error)
}

// ---------------------------------------------------------------------------
// Service — public API
// ---------------------------------------------------------------------------

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
		Metrics: buildComparisonMetrics(row, comparison.BaselineVersion != nil),
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
	return buildTrafficResponse(comparison, rows, req.Version), nil
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
	return buildErrorChangesResponse(comparison, rows), nil
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
	return buildDimensionDiffResponse(comparison, rows), nil
}

// ---------------------------------------------------------------------------
// Context resolution — determines the comparison window for a deployment
// ---------------------------------------------------------------------------

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
	candidates := filterAndSortCandidates(rows, req.Service, req.Environment)

	target := indexOfVersion(candidates, req.Version)
	if target < 0 {
		return models.Context{}, errorcode.NotFoundError{Msg: "deployment not found in the selected range"}
	}

	var nextFirstSeen *time.Time
	if target+1 < len(candidates) {
		nextFirstSeen = &candidates[target+1].FirstSeen
	}
	window, err := computeWindow(
		candidates[target].FirstSeen,
		nextFirstSeen,
		time.UnixMilli(req.StartMs),
		time.UnixMilli(req.EndMs),
	)
	if err != nil {
		return models.Context{}, err
	}

	out := models.Context{
		Service:     req.Service,
		Environment: req.Environment,
		Version:     req.Version,
		FirstSeen:   candidates[target].FirstSeen,
		Window:      window,
	}
	if target > 0 {
		out.BaselineVersion = pointer(candidates[target-1].Version)
	}
	return out, nil
}

// indexOfVersion returns the index of the first candidate matching version, or -1.
func indexOfVersion(candidates []models.RawDeploymentRow, version string) int {
	for i := range candidates {
		if candidates[i].Version == version {
			return i
		}
	}
	return -1
}

// filterAndSortCandidates returns rows matching the given service and
// environment, sorted by FirstSeen (ties broken by Version).
func filterAndSortCandidates(rows []models.RawDeploymentRow, service, environment string) []models.RawDeploymentRow {
	candidates := make([]models.RawDeploymentRow, 0, len(rows))
	for _, row := range rows {
		if row.Service == service && row.Environment == environment {
			candidates = append(candidates, row)
		}
	}
	sortByFirstSeen(candidates)
	return candidates
}

// computeWindow builds an equal-length comparison window clamped to the
// user's picker range [pickerStart, pickerEnd].
//
// Current window:  [max(firstSeen, pickerStart), min(nextFirstSeen, pickerEnd)]
// Baseline window: mirror of equal duration immediately before current start.
func computeWindow(firstSeen time.Time, nextFirstSeen *time.Time, pickerStart, pickerEnd time.Time) (models.Window, error) {
	windowStart := laterOf(firstSeen, pickerStart)

	windowEnd := pickerEnd
	if nextFirstSeen != nil && nextFirstSeen.Before(windowEnd) {
		windowEnd = *nextFirstSeen
	}

	duration := windowEnd.Sub(windowStart)
	if duration <= 0 {
		return models.Window{}, errorcode.ValidationError{Msg: "deployment has no comparable traffic window"}
	}

	return models.Window{
		CurrentStart:  windowStart,
		CurrentEnd:    windowEnd,
		BaselineStart: windowStart.Add(-duration),
		BaselineEnd:   windowStart,
	}, nil
}

// ---------------------------------------------------------------------------
// List response builder
// ---------------------------------------------------------------------------

func buildListResponse(rows []models.RawDeploymentRow, endMs int64) models.ListResponse {
	grouped, environmentSet, serviceSet := groupRowsByKey(rows)

	results := make([]models.Deployment, 0, len(rows))
	var latest *time.Time
	for _, group := range grouped {
		sortByFirstSeen(group)
		items, groupLatest := buildDeploymentGroup(group, endMs)
		results = append(results, items...)
		if groupLatest != nil && (latest == nil || groupLatest.After(*latest)) {
			latest = groupLatest
		}
	}
	sortDeploymentResults(results)

	environments := sortedKeys(environmentSet)
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

// groupRowsByKey buckets rows by "service\x00environment" and collects the
// distinct environment and service sets.
func groupRowsByKey(rows []models.RawDeploymentRow) (
	grouped map[string][]models.RawDeploymentRow,
	environmentSet map[string]struct{},
	serviceSet map[string]struct{},
) {
	grouped = make(map[string][]models.RawDeploymentRow)
	environmentSet = make(map[string]struct{})
	serviceSet = make(map[string]struct{})
	for _, row := range rows {
		key := row.Service + "\x00" + row.Environment
		grouped[key] = append(grouped[key], row)
		environmentSet[row.Environment] = struct{}{}
		serviceSet[row.Service] = struct{}{}
	}
	return grouped, environmentSet, serviceSet
}

// buildDeploymentGroup converts a pre-sorted group of raw rows (same service +
// environment) into Deployment response items with delta annotations. Returns
// the latest FirstSeen within the group.
func buildDeploymentGroup(group []models.RawDeploymentRow, endMs int64) ([]models.Deployment, *time.Time) {
	totalRequests := sumRequests(group)
	items := make([]models.Deployment, len(group))
	var latest *time.Time

	for i, row := range group {
		items[i] = models.Deployment{
			Service:      row.Service,
			Environment:  row.Environment,
			Version:      row.Version,
			FirstSeen:    row.FirstSeen,
			TimelineEnd:  time.UnixMilli(endMs),
			TrafficShare: metrics.Percentage(row.Requests, totalRequests),
			RequestCount: row.Requests,
			ErrorRate:    metrics.Percentage(row.Errors, row.Requests),
			P95Ms:        p95From(row.QS),
		}
		if i+1 < len(group) {
			items[i].TimelineEnd = group[i+1].FirstSeen
		}
		if i > 0 {
			attachDeltas(&items[i], group[i-1])
		}
		if latest == nil || row.FirstSeen.After(*latest) {
			value := row.FirstSeen
			latest = &value
		}
	}
	return items, latest
}

// attachDeltas sets PreviousVersion and metric deltas on item relative to
// the previous deployment row in the same group.
func attachDeltas(item *models.Deployment, previous models.RawDeploymentRow) {
	previousVersion := previous.Version
	previousErrorRate := metrics.Percentage(previous.Errors, previous.Requests)
	previousP95 := p95From(previous.QS)
	item.PreviousVersion = &previousVersion
	item.ErrorRateDelta = pointer(item.ErrorRate - previousErrorRate)
	item.P95DeltaMs = pointer(item.P95Ms - previousP95)
}

func sumRequests(group []models.RawDeploymentRow) uint64 {
	var total uint64
	for _, row := range group {
		total += row.Requests
	}
	return total
}

// sortDeploymentResults orders the final result set: newest first, then
// alphabetically by service, environment, and version.
func sortDeploymentResults(results []models.Deployment) {
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
}

// ---------------------------------------------------------------------------
// Traffic response builder
// ---------------------------------------------------------------------------

func buildTrafficResponse(
	comparison models.Context,
	rows []models.RawTrafficRow,
	requestedVersion string,
) models.TrafficResponse {
	startMs := comparison.Window.BaselineStart.UnixMilli()
	endMs := comparison.Window.CurrentEnd.UnixMilli()
	grain := timebucket.DisplayGrainForRange(startMs, endMs)

	versions, buckets, series := timebucket.FillGapsKeyed(
		startMs, endMs, grain, rows,
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
	sortTrafficSeries(out.Series, requestedVersion)
	return out
}

// sortTrafficSeries places the requested version first, then sorts the rest
// alphabetically.
func sortTrafficSeries(series []models.TrafficSeries, requestedVersion string) {
	sort.SliceStable(series, func(i, j int) bool {
		if series[i].Version == requestedVersion {
			return true
		}
		if series[j].Version == requestedVersion {
			return false
		}
		return series[i].Version < series[j].Version
	})
}

// ---------------------------------------------------------------------------
// Error changes response builder
// ---------------------------------------------------------------------------

func buildErrorChangesResponse(
	comparison models.Context,
	rows []models.RawErrorChangeRow,
) models.ErrorChangesResponse {
	out := models.ErrorChangesResponse{
		Context:  comparison,
		New:      make([]models.ErrorChange, 0, len(rows)),
		Resolved: make([]models.ErrorChange, 0, len(rows)),
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
	return out
}

// ---------------------------------------------------------------------------
// Dimension diff response builder
// ---------------------------------------------------------------------------

func buildDimensionDiffResponse(
	comparison models.Context,
	rows []models.RawDimensionDiffRow,
) models.DimensionDiffResponse {
	hasBaseline := comparison.BaselineVersion != nil
	results := make([]models.DimensionDiff, len(rows))
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
	return models.DimensionDiffResponse{Context: comparison, Results: results}
}

// ---------------------------------------------------------------------------
// Comparison metrics builder
// ---------------------------------------------------------------------------

func buildComparisonMetrics(row models.RawComparisonRow, hasBaseline bool) models.ComparisonMetrics {
	currentLatency := quantiles(row.CurrentQS)
	baselineLatency := quantiles(row.BaselineQS)
	return models.ComparisonMetrics{
		Requests:  compareMetric(float64(row.CurrentRequests), float64(row.BaselineRequests), hasBaseline),
		Errors:    compareMetric(float64(row.CurrentErrors), float64(row.BaselineErrors), hasBaseline),
		ErrorRate: compareMetric(metrics.Percentage(row.CurrentErrors, row.CurrentRequests), metrics.Percentage(row.BaselineErrors, row.BaselineRequests), hasBaseline),
		P50Ms:     compareMetric(currentLatency[0], baselineLatency[0], hasBaseline),
		P75Ms:     compareMetric(currentLatency[1], baselineLatency[1], hasBaseline),
		P90Ms:     compareMetric(currentLatency[2], baselineLatency[2], hasBaseline),
		P95Ms:     compareMetric(currentLatency[3], baselineLatency[3], hasBaseline),
		P99Ms:     compareMetric(currentLatency[4], baselineLatency[4], hasBaseline),
	}
}

func compareMetric(current, baseline float64, hasBaseline bool) models.MetricComparison {
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

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

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

// sortByFirstSeen sorts deployment rows by FirstSeen ascending, breaking ties
// by Version. Used consistently across list and detail flows.
func sortByFirstSeen(rows []models.RawDeploymentRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].FirstSeen.Equal(rows[j].FirstSeen) {
			return rows[i].Version < rows[j].Version
		}
		return rows[i].FirstSeen.Before(rows[j].FirstSeen)
	})
}

// sortedKeys returns the keys of a set in sorted order.
func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// laterOf returns whichever of a or b is later.
func laterOf(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func pointer[T any](value T) *T {
	return &value
}

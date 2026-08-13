package explorer

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/optikklabs/query/internal/modules/metrics/filter"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"golang.org/x/sync/errgroup"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListMetricNames(ctx context.Context, tenantID, startMs, endMs int64, search string) ([]MetricNameEntry, error) {
	rows, err := s.repo.ListMetricNames(ctx, tenantID, startMs, endMs, search)
	if err != nil {
		return nil, err
	}
	out := make([]MetricNameEntry, len(rows))
	for i, row := range rows {
		metricType := normalizeMetricType(row.MetricType)
		if metricType == "" {
			return nil, fmt.Errorf("unsupported metric type %q", row.MetricType)
		}
		out[i] = MetricNameEntry{
			Name:        row.MetricName,
			Type:        metricType,
			Unit:        row.Unit,
			Description: row.Description,
			Temporality: row.Temporality,
			IsMonotonic: row.IsMonotonic,
		}
	}
	return out, nil
}

func (s *Service) ListTagValues(ctx context.Context, tenantID, startMs, endMs int64, metricName, tagKey string) ([]TagValueResult, error) {
	if !filter.ValidKey(tagKey) {
		return nil, errorcode.ValidationError{Msg: fmt.Sprintf("invalid tag key %q", tagKey)}
	}
	var rows []tagValueDTO
	var err error
	if canonical := filter.Canonical(tagKey); canonical != "" {
		rows, err = s.repo.ListResourceTagValues(ctx, tenantID, startMs, endMs, metricName, canonical)
	} else {
		rows, err = s.repo.ListAttributeTagValues(ctx, tenantID, startMs, endMs, metricName, tagKey)
	}
	if err != nil {
		return nil, err
	}
	out := make([]TagValueResult, len(rows))
	for i, row := range rows {
		out[i] = TagValueResult(row)
	}
	return out, nil
}

var staticResourceKeys = []string{
	"service", "host", "pod", "container", "environment", "k8s_namespace",
	"k8s.node.name", "cloud.provider", "cloud.account.id", "cloud.region", "cloud.platform",
}

func (s *Service) ListTags(ctx context.Context, tenantID, startMs, endMs int64, metricName, tagKey string) ([]TagEntry, error) {
	if tagKey != "" {
		values, err := s.ListTagValues(ctx, tenantID, startMs, endMs, metricName, tagKey)
		if err != nil {
			return nil, err
		}
		vals := make([]string, len(values))
		for i, v := range values {
			vals[i] = v.TagValue
		}
		return []TagEntry{{Key: tagKey, Values: vals}}, nil
	}

	dynamicKeys, err := s.repo.ListMetricTagKeys(ctx, tenantID, startMs, endMs, metricName)
	if err != nil {
		return nil, err
	}

	keySet := make(map[string]struct{}, len(staticResourceKeys)+len(dynamicKeys))
	for _, k := range staticResourceKeys {
		keySet[k] = struct{}{}
	}
	for _, k := range dynamicKeys {
		if k != "" {
			keySet[k] = struct{}{}
		}
	}

	keys := slices.Sorted(maps.Keys(keySet))

	tags := make([]TagEntry, len(keys))
	for i, key := range keys {
		tags[i] = TagEntry{Key: key, Values: []string{}}
	}
	return tags, nil
}

type preparedQuery struct {
	request    MetricQuery
	filter     filter.Filters
	metricType string
	result     QueryResult
}

func (s *Service) Query(ctx context.Context, tenantID int64, req QueryRequest) (*QueryResponse, error) {
	prepared, metricNames, err := prepareQueries(tenantID, req)
	if err != nil {
		return nil, err
	}
	if err := s.resolveMetricKinds(ctx, tenantID, req, prepared, metricNames); err != nil {
		return nil, err
	}
	if err := s.executeQueries(ctx, prepared); err != nil {
		return nil, err
	}
	return collectResults(prepared), nil
}

func prepareQueries(tenantID int64, req QueryRequest) ([]preparedQuery, []string, error) {
	prepared := make([]preparedQuery, len(req.Queries))
	metricNames := make([]string, 0, len(req.Queries))
	seenMetrics := make(map[string]struct{}, len(req.Queries))
	for i, query := range req.Queries {
		queryFilter := toFilter(tenantID, req.StartTime, req.EndTime, req.Step, query)
		if err := queryFilter.Validate(); err != nil {
			return nil, nil, errorcode.ValidationError{Msg: fmt.Sprintf("query %q: %v", query.ID, err)}
		}
		prepared[i] = preparedQuery{request: query, filter: queryFilter}
		if _, seen := seenMetrics[query.MetricName]; !seen {
			seenMetrics[query.MetricName] = struct{}{}
			metricNames = append(metricNames, query.MetricName)
		}
	}
	return prepared, metricNames, nil
}

func (s *Service) resolveMetricKinds(ctx context.Context, tenantID int64, req QueryRequest, prepared []preparedQuery, metricNames []string) error {
	kinds, err := s.repo.ResolveMetricKinds(ctx, tenantID, req.StartTime, req.EndTime, metricNames)
	if err != nil {
		return fmt.Errorf("resolve metric metadata: %w", err)
	}
	for i := range prepared {
		query := &prepared[i]
		kind, found := kinds[query.filter.MetricName]
		if !found {
			return errorcode.ValidationError{Msg: fmt.Sprintf("query %q: metric metadata is unavailable", query.request.ID)}
		}
		cumulative, histogram, err := resolveMetricKind(kind)
		if err != nil {
			return errorcode.ValidationError{Msg: fmt.Sprintf("query %q: %v", query.request.ID, err)}
		}
		if err := validateAggregationForMode(query.filter.Aggregation, cumulative, histogram); err != nil {
			return errorcode.ValidationError{Msg: fmt.Sprintf("query %q: %v", query.request.ID, err)}
		}
		query.filter.Cumulative = cumulative
		query.filter.Histogram = histogram
		query.metricType = kind.MetricType
	}
	return nil
}

func (s *Service) executeQueries(ctx context.Context, prepared []preparedQuery) error {
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(4)
	for i := range prepared {
		group.Go(func() error {
			query := &prepared[i]
			queryFilter := query.filter
			rows, err := s.repo.QueryRollupSeries(groupCtx, queryFilter)
			if err != nil {
				return fmt.Errorf("query %q: %w", query.request.ID, err)
			}

			fillZero := shouldZeroFill(query.metricType, queryFilter.Aggregation, queryFilter.Cumulative)
			points := applyAggregation(rows, queryFilter.Aggregation, queryFilter.StartMs, queryFilter.EndMs, queryFilter.Step, queryFilter.Cumulative, queryFilter.Histogram)
			query.result = buildGroupedColumnarResult(
				rows, points, queryFilter.GroupBy,
				queryFilter.StartMs, queryFilter.EndMs, queryFilter.Step, fillZero,
			)
			return nil
		})
	}
	return group.Wait()
}

func collectResults(prepared []preparedQuery) *QueryResponse {
	results := make(map[string]QueryResult, len(prepared))
	for _, query := range prepared {
		results[query.request.ID] = query.result
	}
	return &QueryResponse{Results: results}
}

package explorer

import (
	"context"
	"fmt"
	"sort"

	"github.com/optikklabs/query/internal/modules/metrics/filter"
	"golang.org/x/sync/errgroup"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListMetricNames(ctx context.Context, tenantID, startMs, endMs int64, search string) ([]MetricNameResult, error) {
	rows, err := s.repo.ListMetricNames(ctx, tenantID, startMs, endMs, search)
	if err != nil {
		return nil, err
	}
	out := make([]MetricNameResult, len(rows))
	for i, row := range rows {
		out[i] = MetricNameResult{
			MetricName:  row.MetricName,
			MetricType:  normalizeMetricType(row.MetricType),
			Unit:        row.Unit,
			Description: row.Description,
		}
	}
	return out, nil
}

func (s *Service) ListTagKeys(ctx context.Context, tenantID, startMs, endMs int64, metricName string) ([]TagKeyResult, error) {
	rows, err := s.repo.ListAttributeTagKeys(ctx, tenantID, startMs, endMs, metricName)
	if err != nil {
		return nil, err
	}

	staticKeys := []TagKeyResult{
		{TagKey: "service"},
		{TagKey: "host"},
		{TagKey: "environment"},
		{TagKey: "k8s_namespace"},
	}

	seen := make(map[string]bool)
	var out []TagKeyResult
	for _, sk := range staticKeys {
		seen[sk.TagKey] = true
		out = append(out, sk)
	}
	for _, row := range rows {
		if !seen[row.TagKey] {
			seen[row.TagKey] = true
			out = append(out, TagKeyResult{TagKey: row.TagKey})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].TagKey < out[j].TagKey
	})

	return out, nil
}

func (s *Service) ListTagValues(ctx context.Context, tenantID, startMs, endMs int64, metricName, tagKey string) ([]TagValueResult, error) {
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
		out[i] = TagValueResult{TagValue: row.TagValue, Count: row.Count}
	}
	return out, nil
}

func (s *Service) ListTags(ctx context.Context, tenantID, startMs, endMs int64, metricName, tagKey string) ([]FETagEntry, error) {
	if tagKey != "" {
		values, err := s.ListTagValues(ctx, tenantID, startMs, endMs, metricName, tagKey)
		if err != nil {
			return nil, err
		}
		vals := make([]string, len(values))
		for i, v := range values {
			vals[i] = v.TagValue
		}
		return []FETagEntry{{Key: tagKey, Values: vals}}, nil
	}

	keys, err := s.ListTagKeys(ctx, tenantID, startMs, endMs, metricName)
	if err != nil {
		return nil, err
	}

	keyNames := make([]string, len(keys))
	for i, k := range keys {
		keyNames[i] = k.TagKey
	}

	rows, err := s.repo.ListTagValuesForKeys(ctx, tenantID, startMs, endMs, metricName, keyNames)
	if err != nil {
		return nil, err
	}

	valuesByKey := make(map[string][]string, len(keys))
	for _, row := range rows {
		valuesByKey[row.TagKey] = append(valuesByKey[row.TagKey], row.TagValue)
	}

	tags := make([]FETagEntry, len(keys))
	for i, k := range keys {
		vals := valuesByKey[k.TagKey]
		if vals == nil {

			vals = []string{}
		}
		tags[i] = FETagEntry{Key: k.TagKey, Values: vals}
	}
	return tags, nil
}

func (s *Service) Query(ctx context.Context, tenantID int64, req FEQueryRequest) (*FEQueryResponse, error) {
	type preparedQuery struct {
		request FEMetricQuery
		filter  filter.Filters
		result  FEQueryResult
	}
	prepared := make([]preparedQuery, len(req.Queries))
	metricNames := make([]string, 0, len(req.Queries))
	seenMetrics := make(map[string]struct{}, len(req.Queries))
	for i, feq := range req.Queries {
		f := convertFEQuery(tenantID, req.StartTime, req.EndTime, req.Step, feq)
		if err := f.Validate(); err != nil {
			return nil, fmt.Errorf("query %q: %w", feq.ID, err)
		}
		prepared[i] = preparedQuery{request: feq, filter: f}
		if _, seen := seenMetrics[f.MetricName]; !seen {
			seenMetrics[f.MetricName] = struct{}{}
			metricNames = append(metricNames, f.MetricName)
		}
	}

	kinds, err := s.repo.ResolveSeriesKinds(ctx, tenantID, req.StartTime, req.EndTime, metricNames)
	if err != nil {
		return nil, fmt.Errorf("resolve metric types: %w", err)
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(4)
	for i := range prepared {
		group.Go(func() error {
			query := &prepared[i]
			kind, found := kinds[query.filter.MetricName]
			var kindPtr *metricKindDTO
			if found {
				kindPtr = &kind
			}
			query.filter.Cumulative, query.filter.Histogram = resolveSeriesFlags(kindPtr)
			metricType := metricTypeFrom(kindPtr)
			fillZero := shouldZeroFill(
				metricType, query.filter.Aggregation, query.filter.Cumulative,
			)

			rows, err := s.repo.QueryRollupSeries(groupCtx, query.filter)
			if err != nil {
				return fmt.Errorf("query %q: %w", query.request.ID, err)
			}

			points := applyAggregation(rows, query.filter.Aggregation, query.filter.StartMs, query.filter.EndMs, query.filter.Step, query.filter.Cumulative, query.filter.Histogram)
			query.result = buildGroupedColumnarResult(
				rows, points, query.request.GroupBy,
				query.filter.StartMs, query.filter.EndMs, query.filter.Step, fillZero,
			)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	results := make(map[string]FEQueryResult, len(prepared))
	for _, query := range prepared {
		results[query.request.ID] = query.result
	}

	return &FEQueryResponse{Results: results}, nil
}

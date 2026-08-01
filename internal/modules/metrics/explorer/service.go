package explorer

import (
	"context"
	"fmt"
	"sort"
	"time"

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

func (s *Service) ListMetricNames(ctx context.Context, tenantID, startMs, endMs int64, search string) ([]FEMetricNameEntry, error) {
	rows, err := s.repo.ListMetricNames(ctx, tenantID, startMs, endMs, search)
	if err != nil {
		return nil, err
	}
	out := make([]FEMetricNameEntry, len(rows))
	for i, row := range rows {
		metricType := normalizeMetricType(row.MetricType)
		if metricType == "" {
			return nil, fmt.Errorf("unsupported metric type %q", row.MetricType)
		}
		out[i] = FEMetricNameEntry{
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
		out[i] = TagValueResult{TagValue: row.TagValue, Count: row.Count}
	}
	return out, nil
}

var staticResourceKeys = []string{
	"service", "host", "pod", "container", "environment", "k8s_namespace",
	"k8s.node.name", "cloud.provider", "cloud.account.id", "cloud.region", "cloud.platform",
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

	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	tags := make([]FETagEntry, len(keys))
	for i, key := range keys {
		tags[i] = FETagEntry{Key: key, Values: []string{}}
	}
	return tags, nil
}

func (s *Service) Query(ctx context.Context, tenantID int64, req FEQueryRequest) (*FEQueryResponse, error) {
	type preparedQuery struct {
		request    FEMetricQuery
		filter     filter.Filters
		metricType string
		result     FEQueryResult
	}
	prepared := make([]preparedQuery, len(req.Queries))
	metricNames := make([]string, 0, len(req.Queries))
	seenMetrics := make(map[string]struct{}, len(req.Queries))
	for i, feq := range req.Queries {
		queryFilter := convertFEQuery(tenantID, req.StartTime, req.EndTime, req.Step, feq)
		if err := queryFilter.Validate(); err != nil {
			return nil, errorcode.ValidationError{Msg: fmt.Sprintf("query %q: %v", feq.ID, err)}
		}
		prepared[i] = preparedQuery{request: feq, filter: queryFilter}
		if _, seen := seenMetrics[feq.MetricName]; !seen {
			seenMetrics[feq.MetricName] = struct{}{}
			metricNames = append(metricNames, feq.MetricName)
		}
	}

	kinds, err := s.repo.ResolveMetricKinds(ctx, tenantID, req.StartTime, req.EndTime, metricNames)
	if err != nil {
		return nil, fmt.Errorf("resolve metric metadata: %w", err)
	}
	for i := range prepared {
		query := &prepared[i]
		kind, found := kinds[query.filter.MetricName]
		if !found {
			return nil, errorcode.ValidationError{Msg: fmt.Sprintf("query %q: metric metadata is unavailable", query.request.ID)}
		}
		cumulative, histogram, err := resolveMetricKind(kind)
		if err != nil {
			return nil, errorcode.ValidationError{Msg: fmt.Sprintf("query %q: %v", query.request.ID, err)}
		}
		if err := validateAggregationForMode(query.filter.Aggregation, cumulative, histogram); err != nil {
			return nil, errorcode.ValidationError{Msg: fmt.Sprintf("query %q: %v", query.request.ID, err)}
		}
		query.filter.Cumulative = cumulative
		query.filter.Histogram = histogram
		query.metricType = kind.MetricType
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(4)
	for i := range prepared {
		group.Go(func() error {
			query := &prepared[i]
			queryFilter := query.filter

			if queryFilter.Cumulative && queryFilter.StartMs < time.Now().Add(-48*time.Hour).UnixMilli() {
				return errorcode.ValidationError{Msg: fmt.Sprintf("query %q: exact cumulative data is retained for 48 hours", query.request.ID)}
			}

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
	if err := group.Wait(); err != nil {
		return nil, err
	}

	results := make(map[string]FEQueryResult, len(prepared))
	for _, query := range prepared {
		results[query.request.ID] = query.result
	}

	return &FEQueryResponse{Results: results}, nil
}

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

	rows, err := s.repo.ListTagValuesAllKeys(ctx, tenantID, startMs, endMs, metricName)
	if err != nil {
		return nil, err
	}

	valuesByKey := make(map[string][]string)
	for _, row := range rows {
		valuesByKey[row.TagKey] = append(valuesByKey[row.TagKey], row.TagValue)
	}
	// Static resource keys always appear, even with no values in range.
	for _, static := range []string{"service", "host", "pod", "container", "environment", "k8s_namespace",
		"k8s.node.name", "cloud.provider", "cloud.account.id", "cloud.region", "cloud.platform"} {
		if _, ok := valuesByKey[static]; !ok {
			valuesByKey[static] = []string{}
		}
	}

	keys := make([]string, 0, len(valuesByKey))
	for key := range valuesByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	tags := make([]FETagEntry, len(keys))
	for i, key := range keys {
		tags[i] = FETagEntry{Key: key, Values: valuesByKey[key]}
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
			if err := validateAggregationForKind(kindPtr, query.filter.Aggregation); err != nil {
				return errorcode.ValidationError{Msg: fmt.Sprintf("query %q: %v", query.request.ID, err)}
			}
			query.filter.Cumulative, query.filter.Histogram = resolveSeriesFlags(kindPtr)
			if query.filter.Cumulative && query.filter.StartMs < time.Now().Add(-48*time.Hour).UnixMilli() {
				return errorcode.ValidationError{Msg: fmt.Sprintf("query %q: exact cumulative data is retained for 48 hours", query.request.ID)}
			}
			metricType := ""
			if kindPtr != nil {
				metricType = kindPtr.MetricType
			}
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

package explorer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

var scalarFields = map[string]struct{}{
	"service":     {},
	"operation":   {},
	"http_method": {},
	"http_status": {},
	"status":      {},
	"environment": {},
}

func IsScalarField(field string) bool {
	_, ok := scalarFields[field]
	return ok
}

type TraceRepository interface {
	Query(ctx context.Context, req QueryRequest) ([]traceIndexRowDTO, error)
	EnrichTraces(ctx context.Context, tenantID int64, traceIDs []string, start, end time.Time) ([]traceAggRow, error)
	QueryFacets(ctx context.Context, req FacetsRequest) ([]facetDimRow, error)
	QueryTrend(ctx context.Context, req TrendRequest) ([]trendRow, error)
	SuggestAttribute(ctx context.Context, tenantID int64, start, end int64, attrKey, prefix string, limit int) ([]suggestionRow, error)
	SuggestScalar(ctx context.Context, tenantID int64, start, end int64, field, prefix string, limit int) ([]suggestionRow, error)
}

type Service struct {
	repo TraceRepository
}

func NewService(repo TraceRepository) *Service {
	return &Service{repo: repo}
}

// enrichSlack pads the page's root-span time span so trace-level aggregation
// still sees spans whose clocks skew slightly past the root's window.
const enrichSlack = 5 * time.Minute

func (s *Service) Query(ctx context.Context, req QueryRequest) (QueryResponse, error) {
	limit := filterutil.PickLimit(req.Limit, 50, 500)
	req.Limit = limit
	rows, err := s.repo.Query(ctx, req)
	if err != nil {
		return QueryResponse{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	aggs, err := s.enrichPage(ctx, req.TenantID, rows)
	if err != nil {
		return QueryResponse{}, err
	}
	return QueryResponse{
		Results:  mapTraces(rows, aggs),
		PageInfo: buildPageInfo(rows, hasMore, limit),
	}, nil
}

// enrichPage aggregates trace-level facts for the page in one bounded round
// trip. The bound is the page's own root-span span, padded by enrichSlack, so
// the spans primary key prunes to a couple of partitions.
func (s *Service) enrichPage(ctx context.Context, tenantID int64, rows []traceIndexRowDTO) (map[string]traceAggRow, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	ids := traceIDsOf(rows)
	start, end := pageBounds(rows)
	aggList, err := s.repo.EnrichTraces(ctx, tenantID, ids, start.Add(-enrichSlack), end.Add(enrichSlack))
	if err != nil {
		slog.ErrorContext(ctx, "explorer: enrichPage failed", slog.Any("error", err), slog.Int64("tenant_id", tenantID))
		return nil, err
	}
	aggs := make(map[string]traceAggRow, len(aggList))
	for _, a := range aggList {
		aggs[a.TraceID] = a
	}
	return aggs, nil
}

// pageBounds returns the earliest root start and latest root end on the page.
func pageBounds(rows []traceIndexRowDTO) (start, end time.Time) {
	start = rows[0].StartTime
	end = rows[0].StartTime
	for _, r := range rows {
		if r.StartTime.Before(start) {
			start = r.StartTime
		}
		rowEnd := r.StartTime.Add(time.Duration(r.DurationNs))
		if rowEnd.After(end) {
			end = rowEnd
		}
	}
	return start, end
}

func traceIDsOf(rows []traceIndexRowDTO) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.TraceID
	}
	return ids
}

func buildPageInfo(rows []traceIndexRowDTO, hasMore bool, limit int) PageInfo {
	info := PageInfo{HasMore: hasMore, Limit: limit}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		info.NextCursor = TraceCursor{StartNs: uint64(last.StartTime.UnixNano()), SpanID: last.SpanID}.Encode()
	}
	return info
}

// mapTrace merges the root span with its trace-level aggregate. The aggregate
// is authoritative for span/error counts, services and the real end time; the
// root row is authoritative for the entry-point fields.
func mapTrace(d traceIndexRowDTO, agg traceAggRow, ok bool) Trace {
	t := Trace{
		TraceID:        d.TraceID,
		StartMs:        uint64(d.StartTime.UnixMilli()),
		EndMs:          uint64(d.StartTime.UnixMilli()),
		DurationMs:     float64(d.DurationNs) / 1_000_000,
		RootService:    d.RootService,
		RootOperation:  d.RootOperation,
		RootStatus:     d.RootStatus,
		RootHTTPMethod: d.RootHTTPMethod,
		RootHTTPStatus: d.RootHTTPStatus,
		SpanCount:      1,
		ServiceSet:     []string{d.RootService},
	}
	if !ok {
		return t
	}
	t.StartMs = uint64(agg.StartTime.UnixMilli())
	t.EndMs = uint64(agg.EndTime.UnixMilli())
	t.DurationMs = float64(agg.EndTime.Sub(agg.StartTime).Nanoseconds()) / 1_000_000
	t.SpanCount = uint32(agg.SpanCount)
	t.ErrorCount = uint32(agg.ErrorCount)
	t.HasError = agg.ErrorCount > 0
	t.ServiceSet = agg.ServiceSet
	return t
}

func mapTraces(rows []traceIndexRowDTO, aggs map[string]traceAggRow) []Trace {
	out := make([]Trace, len(rows))
	for i, r := range rows {
		agg, ok := aggs[r.TraceID]
		out[i] = mapTrace(r, agg, ok)
	}
	return out
}

func (s *Service) QueryFacets(ctx context.Context, req FacetsRequest) (Facets, error) {
	rows, err := s.repo.QueryFacets(ctx, req)
	if err != nil {
		return Facets{}, err
	}
	return pivotFacets(rows), nil
}

func pivotFacets(rows []facetDimRow) Facets {
	var f Facets
	for _, row := range rows {
		b := FacetBucket{Value: row.Value, Count: row.Count}
		switch row.Dim {
		case "service":
			f.Service = append(f.Service, b)
		case "operation":
			f.Operation = append(f.Operation, b)
		case "http_method":
			f.HTTPMethod = append(f.HTTPMethod, b)
		case "http_status":
			f.HTTPStatus = append(f.HTTPStatus, b)
		case "status":
			f.Status = append(f.Status, b)
		}
	}
	return f
}

func (s *Service) QueryTrend(ctx context.Context, req TrendRequest) ([]TrendBucket, error) {
	rows, err := s.repo.QueryTrend(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make([]TrendBucket, len(rows))
	for i, r := range rows {
		out[i] = TrendBucket{
			TimeBucket: timebucket.FormatDisplayBucket(r.TimeBucket),
			Total:      r.Total,
			Errors:     r.Errors,
		}
	}
	return out, nil
}

func (s *Service) Suggest(ctx context.Context, req SuggestRequest, tenantID int64) (SuggestResponse, error) {
	limit := filterutil.PickLimit(req.Limit, 10, 50)
	rows, err := s.fetchSuggest(ctx, tenantID, req, limit)
	if err != nil {
		slog.ErrorContext(ctx, "suggest: Suggest failed", slog.Any("error", err), slog.Int64("tenant_id", tenantID), slog.String("field", req.Field))
		return SuggestResponse{}, err
	}
	return SuggestResponse{Suggestions: rows}, nil
}

func (s *Service) fetchSuggest(ctx context.Context, tenantID int64, req SuggestRequest, limit int) ([]Suggestion, error) {
	var rows []suggestionRow
	var err error
	switch {
	case strings.HasPrefix(req.Field, "@"):
		rows, err = s.repo.SuggestAttribute(ctx, tenantID, req.StartTime, req.EndTime, req.Field, req.Prefix, limit)
	case !IsScalarField(req.Field):
		return nil, fmt.Errorf("suggest: unknown scalar field %q", req.Field)
	default:
		rows, err = s.repo.SuggestScalar(ctx, tenantID, req.StartTime, req.EndTime, req.Field, req.Prefix, limit)
	}
	if err != nil {
		return nil, err
	}
	return filterutil.MapSuggestionRows(rows), nil
}

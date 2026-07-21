package explorer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

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
	Query(ctx context.Context, req QueryRequest) ([]traceIndexRowDTO, bool, error)
	EnrichTraces(ctx context.Context, tenantID int64, traceIDs []string) (map[string]traceAggRow, error)
	QueryFacets(ctx context.Context, req FacetsRequest) (Facets, error)
	QueryTrend(ctx context.Context, req TrendRequest) ([]TrendBucket, error)
	SuggestAttribute(ctx context.Context, tenantID int64, start, end int64, field, prefix string, limit int) ([]Suggestion, error)
	SuggestScalar(ctx context.Context, tenantID int64, start, end int64, field, prefix string, limit int) ([]Suggestion, error)
}

type Service struct {
	repo TraceRepository
}

func NewService(repo TraceRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Query(ctx context.Context, req QueryRequest) (QueryResponse, error) {
	limit := filterutil.PickLimit(req.Limit, 50, 500)
	req.Limit = limit
	rows, hasMore, err := s.repo.Query(ctx, req)
	if err != nil {
		return QueryResponse{}, err
	}
	// The root scan owns ordering and paging; this fills in the trace-level
	// facts a single root span cannot know.
	aggs, err := s.repo.EnrichTraces(ctx, req.TenantID, traceIDsOf(rows))
	if err != nil {
		return QueryResponse{}, err
	}
	return QueryResponse{
		Results:  mapTraces(rows, aggs),
		PageInfo: buildPageInfo(rows, hasMore, limit),
	}, nil
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
	return s.repo.QueryFacets(ctx, req)
}

func (s *Service) QueryTrend(ctx context.Context, req TrendRequest) ([]TrendBucket, error) {
	return s.repo.QueryTrend(ctx, req)
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
	if strings.HasPrefix(req.Field, "@") {
		return s.repo.SuggestAttribute(ctx, tenantID, req.StartTime, req.EndTime, req.Field, req.Prefix, limit)
	}
	if !IsScalarField(req.Field) {
		return nil, fmt.Errorf("suggest: unknown scalar field %q", req.Field)
	}
	return s.repo.SuggestScalar(ctx, tenantID, req.StartTime, req.EndTime, req.Field, req.Prefix, limit)
}

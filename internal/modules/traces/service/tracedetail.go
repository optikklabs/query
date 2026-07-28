package service

import (
	"context"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"github.com/optikklabs/query/internal/modules/services/topology"
	"github.com/optikklabs/query/internal/modules/traces/models"
	"github.com/optikklabs/query/internal/modules/traces/repository"
)

const (
	// spanListLimit preserves the old /spans and /critical-path caps.
	spanListLimit = 5000
	// errorSpanLimit preserves the old /error-path and /errors caps.
	errorSpanLimit = 1000
)

// TraceDetail is the consolidated trace-detail response: summary plus the
// span list and every view derived from it. Summary is nil when the trace
// has no spans in range (the span list is then empty).
type TraceDetail struct {
	Summary      *models.TraceSummary      `json:"summary"`
	Spans        []models.SpanListItem     `json:"spans"`
	CriticalPath []models.CriticalPathSpan `json:"criticalPath"`
	ErrorPath    []models.ErrorPathSpan    `json:"errorPath"`
	ServiceMap   topology.TopologyResponse `json:"serviceMap"`
	Errors       []models.TraceErrorGroup  `json:"errors"`
}

// GetTraceDetail fetches the trace once (uncapped summary aggregate plus a
// capped span scan) and derives all detail views in memory.
func (s *Service) GetTraceDetail(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) (*TraceDetail, error) {
	var (
		summaryRow *repository.TraceSummaryRow
		rows       []repository.TraceSpanRow
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		summaryRow, err = s.repo.GetTraceSummary(gctx, tenantID, traceID, startMs, endMs)
		return err
	})
	g.Go(func() error {
		var err error
		rows, err = s.repo.ListTraceSpanRows(gctx, tenantID, traceID, startMs, endMs)
		return err
	})
	if err := g.Wait(); err != nil {
		slog.ErrorContext(ctx, "detail: GetTraceDetail failed", slog.Any("error", err), slog.Int64("tenant_id", tenantID), slog.String("trace_id", traceID))
		return nil, err
	}
	return buildTraceDetail(summaryRow, rows), nil
}

func buildTraceDetail(summaryRow *repository.TraceSummaryRow, rows []repository.TraceSpanRow) *TraceDetail {
	listRows := rows
	if len(listRows) > spanListLimit {
		listRows = listRows[:spanListLimit]
	}
	errorRows := filterErrorRows(rows, errorSpanLimit)

	detail := &TraceDetail{
		Spans:        toSpanListItems(listRows),
		CriticalPath: buildCriticalPath(listRows),
		ErrorPath:    buildErrorPath(errorRows),
		ServiceMap:   buildServiceMap(rows),
		Errors:       groupErrors(errorRows),
	}
	if summaryRow != nil {
		summary := foldTraceSummary(*summaryRow)
		summary.Truncated = summaryRow.SpanCount > uint64(len(detail.Spans))
		detail.Summary = &summary
	}
	return detail
}

func toSpanListItems(rows []repository.TraceSpanRow) []models.SpanListItem {
	items := make([]models.SpanListItem, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		items = append(items, models.SpanListItem{
			SpanID:        r.SpanID,
			ParentSpanID:  r.ParentSpanID,
			TraceID:       r.TraceID,
			ServiceName:   r.ServiceName,
			OperationName: r.OperationName,
			KindString:    r.KindString,
			StatusCode:    r.StatusCode,
			HasError:      r.HasError,
			DurationMs:    r.DurationMs(),
			Timestamp:     r.Timestamp,
			StartNs:       r.Timestamp.UnixNano(),
		})
	}
	return items
}

func filterErrorRows(rows []repository.TraceSpanRow, limit int) []repository.TraceSpanRow {
	out := make([]repository.TraceSpanRow, 0)
	for i := range rows {
		if rows[i].HasError {
			out = append(out, rows[i])
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

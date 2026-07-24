package detail

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetTraceSummary(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) (*TraceSummary, error) {
	row, err := s.repo.GetTraceSummary(ctx, tenantID, traceID, startMs, endMs)
	if err != nil {
		slog.ErrorContext(ctx, "detail: GetTraceSummary failed", slog.Any("error", err), slog.Int64("tenant_id", tenantID), slog.String("trace_id", traceID))
		return nil, err
	}
	return row, nil
}

func (s *Service) GetSpanEvents(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) ([]SpanEvent, error) {
	combined, err := s.repo.GetSpanEvents(ctx, tenantID, traceID, startMs, endMs)
	if err != nil {
		slog.ErrorContext(ctx, "detail: GetSpanEvents failed", slog.Any("error", err), slog.Int64("tenant_id", tenantID), slog.String("trace_id", traceID))
		return nil, err
	}
	eventRows, exceptionRows := splitEventRows(combined)

	events := make([]SpanEvent, 0, len(eventRows))
	seenException := make(map[string]bool, len(eventRows))
	for _, row := range eventRows {
		if row.Event.Name == "exception" {
			seenException[row.SpanID] = true
		}

		attrJSON := "{}"
		if len(row.Event.Attributes) > 0 {
			if b, marshalErr := json.Marshal(row.Event.Attributes); marshalErr == nil {
				attrJSON = string(b)
			}
		}

		events = append(events, SpanEvent{
			SpanID:     row.SpanID,
			TraceID:    row.TraceID,
			EventName:  row.Event.Name,
			Timestamp:  time.Unix(0, int64(row.Event.TimeUnixNano)),
			Attributes: attrJSON,
		})
	}

	for _, row := range exceptionRows {
		if seenException[row.SpanID] {
			continue
		}
		attrs := map[string]string{}
		if row.ExceptionType != "" {
			attrs["exception.type"] = row.ExceptionType
		}
		if row.ExceptionMessage != "" {
			attrs["exception.message"] = row.ExceptionMessage
		}
		if row.ExceptionStacktrace != "" {
			attrs["exception.stacktrace"] = row.ExceptionStacktrace
		}
		attrJSON := "{}"
		if len(attrs) > 0 {
			if b, marshalErr := json.Marshal(attrs); marshalErr == nil {
				attrJSON = string(b)
			}
		}
		events = append(events, SpanEvent{
			SpanID:     row.SpanID,
			TraceID:    row.TraceID,
			EventName:  "exception",
			Timestamp:  row.Timestamp,
			Attributes: attrJSON,
		})
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].Timestamp.Equal(events[j].Timestamp) {
			if events[i].SpanID == events[j].SpanID {
				return events[i].EventName < events[j].EventName
			}
			return events[i].SpanID < events[j].SpanID
		}
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
	return events, nil
}

func (s *Service) GetSpanAttributes(ctx context.Context, tenantID int64, traceID, spanID string, startMs, endMs int64) (*SpanAttributes, error) {
	row, err := s.repo.GetSpanAttributes(ctx, tenantID, traceID, spanID, startMs, endMs)
	if err != nil {
		slog.ErrorContext(ctx, "detail: GetSpanAttributes failed", slog.Any("error", err), slog.Int64("tenant_id", tenantID), slog.String("trace_id", traceID), slog.String("span_id", spanID))
		return nil, err
	}
	if row == nil {
		return nil, nil
	}

	attrs := row.Attributes
	if attrs == nil {
		attrs = map[string]string{}
	}
	resourceAttrs := map[string]string{}

	outLinks := make([]SpanLink, 0, len(row.Links))
	for _, l := range row.Links {
		outLinks = append(outLinks, SpanLink{
			TraceID:    l.TraceID,
			SpanID:     l.SpanID,
			TraceState: l.TraceState,
			Attributes: l.Attributes,
		})
	}

	return &SpanAttributes{
		SpanID:                row.SpanID,
		TraceID:               row.TraceID,
		OperationName:         row.OperationName,
		ServiceName:           row.ServiceName,
		AttributesString:      attrs,
		ResourceAttrs:         resourceAttrs,
		Attributes:            attrs,
		ExceptionType:         row.ExceptionType,
		ExceptionMessage:      row.ExceptionMessage,
		ExceptionStacktrace:   row.ExceptionStacktrace,
		DBSystem:              row.DBSystem,
		DBName:                row.DBName,
		DBStatement:           row.DBStatement,
		DBStatementNormalized: normalizeDBStatement(row.DBStatement),
		Links:                 outLinks,
	}, nil
}

func (s *Service) GetRelatedTraces(ctx context.Context, tenantID int64, serviceName, operationName string, startMs, endMs int64, excludeTraceID string, limit int) ([]RelatedTrace, error) {
	return s.repo.GetRelatedTraces(ctx, tenantID, serviceName, operationName, startMs, endMs, excludeTraceID, limit)
}

func (s *Service) ListSpansByTrace(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) ([]SpanListItem, error) {
	rows, err := s.repo.ListSpansByTrace(ctx, tenantID, traceID, startMs, endMs)
	if err != nil {
		return nil, err
	}
	fillStartNs(rows)
	return rows, nil
}

func fillStartNs(items []SpanListItem) {
	for i := range items {
		items[i].StartNs = items[i].Timestamp.UnixNano()
	}
}

type spanEventRow struct {
	SpanID    string
	TraceID   string
	Timestamp time.Time
	Event     spanEventTuple
}

func splitEventRows(rows []spanEventCombinedRow) ([]spanEventRow, []exceptionRow) {
	var events []spanEventRow
	var exceptions []exceptionRow
	for _, r := range rows {
		for _, ev := range r.Events {
			events = append(events, spanEventRow{
				SpanID: r.SpanID, TraceID: r.TraceID, Timestamp: r.Timestamp, Event: ev,
			})
		}
		if r.ExceptionType != "" {
			exceptions = append(exceptions, exceptionRow{
				SpanID: r.SpanID, TraceID: r.TraceID, Timestamp: r.Timestamp,
				ExceptionType: r.ExceptionType, ExceptionMessage: r.ExceptionMessage,
				ExceptionStacktrace: r.ExceptionStacktrace,
			})
		}
	}
	for i, j := 0, len(exceptions)-1; i < j; i, j = i+1, j-1 {
		exceptions[i], exceptions[j] = exceptions[j], exceptions[i]
	}
	return events, exceptions
}

var (
	reNumberLiteral = regexp.MustCompile(`\b\d+(\.\d+)?\b`)
	reStringLiteral = regexp.MustCompile(`'[^']*'`)
	reMultiSpace    = regexp.MustCompile(`\s+`)
)

func normalizeDBStatement(stmt string) string {
	if stmt == "" {
		return ""
	}
	s := reStringLiteral.ReplaceAllString(stmt, "?")
	s = reNumberLiteral.ReplaceAllString(s, "?")
	s = reMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

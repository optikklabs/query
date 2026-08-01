package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/modules/traces/models"
	"github.com/optikklabs/query/internal/modules/traces/repository"
)

func foldTraceSummary(res repository.TraceSummaryRow) models.TraceSummary {
	return models.TraceSummary{
		TraceID:        res.TraceID,
		StartMs:        uint64(res.StartTime.UnixMilli()),
		EndMs:          uint64(res.EndTime.UnixMilli()),
		DurationMs:     float64(res.EndTime.Sub(res.StartTime).Nanoseconds()) / 1_000_000,
		RootService:    res.RootService,
		RootOperation:  res.RootOperation,
		RootStatus:     res.RootStatus,
		RootHTTPMethod: res.RootHTTPMethod,
		RootHTTPStatus: res.RootHTTPStatus,
		SpanCount:      uint32(res.SpanCount),
		HasError:       res.HasError,
		ErrorCount:     uint32(res.ErrorCount),
		ServiceSet:     res.ServiceSet,
		RootMissing:    res.RootMissing,
	}
}

func (s *Service) GetSpanEvents(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) ([]models.SpanEvent, error) {
	combined, err := s.repo.GetSpanEvents(ctx, tenantID, traceID, startMs, endMs)
	if err != nil {
		slog.ErrorContext(ctx, "detail: GetSpanEvents failed", slog.Any("error", err), slog.Int64("tenant_id", tenantID), slog.String("trace_id", traceID))
		return nil, err
	}
	eventRows, exceptionRows := splitEventRows(combined)
	events, seenExceptions := mapExplicitEvents(eventRows)
	for _, row := range exceptionRows {
		if !seenExceptions[row.SpanID] {
			events = append(events, mapExceptionEvent(row))
		}
	}
	sortSpanEvents(events)
	return events, nil
}

func mapExplicitEvents(rows []spanEventRow) ([]models.SpanEvent, map[string]bool) {
	events := make([]models.SpanEvent, 0, len(rows))
	seenException := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row.Event.Name == "exception" {
			seenException[row.SpanID] = true
		}
		events = append(events, models.SpanEvent{
			SpanID:     row.SpanID,
			TraceID:    row.TraceID,
			EventName:  row.Event.Name,
			Timestamp:  time.Unix(0, int64(row.Event.TimeUnixNano)),
			Attributes: marshalAttributes(row.Event.Attributes),
		})
	}
	return events, seenException
}

func mapExceptionEvent(row exceptionRow) models.SpanEvent {
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
	return models.SpanEvent{SpanID: row.SpanID, TraceID: row.TraceID, EventName: "exception", Timestamp: row.Timestamp, Attributes: marshalAttributes(attrs)}
}

func marshalAttributes(attrs map[string]string) string {
	if len(attrs) == 0 {
		return "{}"
	}
	b, err := json.Marshal(attrs)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func sortSpanEvents(events []models.SpanEvent) {
	sort.Slice(events, func(i, j int) bool {
		if events[i].Timestamp.Equal(events[j].Timestamp) {
			if events[i].SpanID == events[j].SpanID {
				return events[i].EventName < events[j].EventName
			}
			return events[i].SpanID < events[j].SpanID
		}
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
}

func (s *Service) GetSpanAttributes(ctx context.Context, tenantID int64, traceID, spanID string, startMs, endMs int64) (*models.SpanAttributes, error) {
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

	outLinks := make([]models.SpanLink, 0, len(row.Links))
	for _, l := range row.Links {
		outLinks = append(outLinks, models.SpanLink{
			TraceID:    l.TraceID,
			SpanID:     l.SpanID,
			TraceState: l.TraceState,
			Attributes: l.Attributes,
		})
	}

	return &models.SpanAttributes{
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

func (s *Service) GetRelatedTraces(ctx context.Context, tenantID int64, serviceName, operationName string, startMs, endMs int64, excludeTraceID string, limit int) ([]models.RelatedTrace, error) {
	return s.repo.GetRelatedTraces(ctx, tenantID, serviceName, operationName, startMs, endMs, excludeTraceID, limit)
}

type spanEventRow struct {
	SpanID    string
	TraceID   string
	Timestamp time.Time
	Event     repository.SpanEventTuple
}

type exceptionRow struct {
	SpanID              string
	TraceID             string
	Timestamp           time.Time
	ExceptionType       string
	ExceptionMessage    string
	ExceptionStacktrace string
}

func splitEventRows(rows []repository.SpanEventCombinedRow) ([]spanEventRow, []exceptionRow) {
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

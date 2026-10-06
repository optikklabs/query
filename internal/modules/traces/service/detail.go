package service

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	dbutil "github.com/optikklabs/query/internal/infra/database"

	"github.com/optikklabs/query/internal/modules/traces/models"
	"github.com/optikklabs/query/internal/modules/traces/repository"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/nullable"
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
		ServiceSet:     nullable.OrEmpty(res.ServiceSet),
		RootMissing:    res.RootMissing,
	}
}

func (s *Service) GetSpanEvents(ctx context.Context, tenantID int64, traceID string, startMs, endMs int64) ([]models.SpanEvent, error) {
	combined, err := s.repo.GetSpanEvents(ctx, tenantID, traceID, startMs, endMs)
	if err != nil {
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
	slices.SortFunc(events, func(a, b models.SpanEvent) int {
		return cmp.Or(a.Timestamp.Compare(b.Timestamp), strings.Compare(a.SpanID, b.SpanID), strings.Compare(a.EventName, b.EventName))
	})
}

func (s *Service) GetSpanAttributes(ctx context.Context, tenantID int64, traceID, spanID string, startMs, endMs int64) (*models.SpanAttributes, error) {
	row, err := s.repo.GetSpanAttributes(ctx, tenantID, traceID, spanID, startMs, endMs)
	if err != nil {
		return nil, dbutil.NoRowsAs(err, errorcode.NotFoundError{Msg: "Span not found"})
	}
	attrs := row.Attributes
	if attrs == nil {
		attrs = map[string]string{}
	}
	links := make([]models.SpanLink, len(row.Links))
	for i, l := range row.Links {
		links[i] = models.SpanLink(l)
	}

	return &models.SpanAttributes{
		SpanID:                row.SpanID,
		TraceID:               row.TraceID,
		OperationName:         row.OperationName,
		ServiceName:           row.ServiceName,
		AttributesString:      attrs,
		ResourceAttrs:         resourceAttributes(&row),
		ExceptionType:         row.ExceptionType,
		ExceptionMessage:      row.ExceptionMessage,
		ExceptionStacktrace:   row.ExceptionStacktrace,
		DBSystem:              row.DBSystem,
		DBName:                row.DBName,
		DBStatement:           row.DBStatement,
		DBStatementNormalized: row.DBStatementNorm,
		Links:                 links,
	}, nil
}

// resourceAttributes rebuilds the resource attributes that ingest promotes
// out of the attribute map into their own columns.
func resourceAttributes(row *repository.SpanAttributeRow) map[string]string {
	out := make(map[string]string, 5)
	for key, value := range map[string]string{
		"service.name":           row.ServiceName,
		"service.version":        row.ServiceVersion,
		"deployment.environment": row.Environment,
		"host.name":              row.Host,
		"k8s.pod.name":           row.Pod,
	} {
		if value != "" {
			out[key] = value
		}
	}
	return out
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
	return events, exceptions
}

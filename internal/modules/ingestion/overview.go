package ingestion

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

func (s *Service) Overview(ctx context.Context, tenantID, startMs, endMs int64) (OverviewResponse, error) {
	var (
		daily       []signalDateCountRow
		serviceRows []serviceUsageRow
		series      []svcCountRow
		cardinality []metricCardinalityRow
	)

	g, groupCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		daily, err = s.repo.DailySignals(groupCtx, tenantID, startMs, endMs)
		return wrapOverviewError("daily usage", err)
	})
	g.Go(func() error {
		var err error
		window := endMs - startMs
		serviceRows, err = s.repo.ServiceUsage(
			groupCtx, tenantID, startMs-window, startMs, endMs,
		)
		return wrapOverviewError("service usage", err)
	})
	g.Go(func() error {
		var err error
		series, err = s.repo.ServiceTimeseries(groupCtx, tenantID, startMs, endMs)
		return wrapOverviewError("service timeseries", err)
	})
	g.Go(func() error {
		var err error
		cardinality, err = s.repo.MetricCardinality(groupCtx, tenantID, startMs, endMs)
		return wrapOverviewError("metric cardinality", err)
	})
	if err := g.Wait(); err != nil {
		return OverviewResponse{}, err
	}

	logs, spans, metrics := splitDailySignals(daily)
	usage := splitServiceUsage(serviceRows)
	active, top := summarizeCardinality(cardinality)
	dates := buildDateAxis(startMs, endMs)
	idx := axisIndex(dates)

	return OverviewResponse{
		Summary:             s.summaryFromUsage(logs, spans, metrics, active, top, startMs, endMs),
		Cost:                s.costFromUsage(logs, spans, metrics, endMs),
		TimeseriesByType:    timeseriesByType(logs, spans, metrics, dates, idx),
		TimeseriesByService: timeseriesByServiceRows(usage.dailyLogs, usage.dailySpans, dates, idx),
		Services: servicesFromUsage(
			usage.logTotals, usage.spanTotals, series,
			usage.priorLogs, usage.priorSpans,
			usage.dailyLogs, usage.dailySpans, startMs, endMs,
		),
		UsageSemantics: "accepted",
	}, nil
}

func wrapOverviewError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("ingestion.Overview %s: %w", operation, err)
}

func splitDailySignals(rows []signalDateCountRow) (logs, spans, metrics []dateCountRow) {
	for _, row := range rows {
		value := dateCountRow{Day: row.Day, Count: row.Count, Bytes: row.Bytes}
		switch row.Signal {
		case "logs":
			logs = append(logs, value)
		case "spans":
			spans = append(spans, value)
		case "metrics":
			metrics = append(metrics, value)
		}
	}
	return logs, spans, metrics
}

type serviceUsageSets struct {
	logTotals, spanTotals []svcCountRow
	priorLogs, priorSpans []svcCountRow
	dailyLogs, dailySpans []svcDateCountRow
}

func splitServiceUsage(rows []serviceUsageRow) serviceUsageSets {
	currentLogs := make(map[string]*svcCountRow)
	currentSpans := make(map[string]*svcCountRow)
	previousLogs := make(map[string]*svcCountRow)
	previousSpans := make(map[string]*svcCountRow)
	var out serviceUsageSets

	for _, row := range rows {
		current := row.Period == "current"
		if current {
			daily := svcDateCountRow{
				Day: row.Day, Service: row.Service,
				Count: row.Count, Bytes: row.Bytes,
			}
			if row.Signal == "logs" {
				out.dailyLogs = append(out.dailyLogs, daily)
			} else if row.Signal == "spans" {
				out.dailySpans = append(out.dailySpans, daily)
			}
		}

		target := usageTotalsMap(row.Signal, current, currentLogs, currentSpans, previousLogs, previousSpans)
		if target == nil {
			continue
		}
		value := target[row.Service]
		if value == nil {
			value = &svcCountRow{Service: row.Service, Env: row.Env}
			target[row.Service] = value
		}
		value.Count += row.Count
		value.Bytes += row.Bytes
		if value.Env == "" {
			value.Env = row.Env
		}
	}

	out.logTotals = mapValues(currentLogs)
	out.spanTotals = mapValues(currentSpans)
	out.priorLogs = mapValues(previousLogs)
	out.priorSpans = mapValues(previousSpans)
	return out
}

func usageTotalsMap(
	signal string,
	current bool,
	currentLogs, currentSpans, previousLogs, previousSpans map[string]*svcCountRow,
) map[string]*svcCountRow {
	switch {
	case current && signal == "logs":
		return currentLogs
	case current && signal == "spans":
		return currentSpans
	case !current && signal == "logs":
		return previousLogs
	case !current && signal == "spans":
		return previousSpans
	default:
		return nil
	}
}

func mapValues(values map[string]*svcCountRow) []svcCountRow {
	rows := make([]svcCountRow, 0, len(values))
	for _, value := range values {
		rows = append(rows, *value)
	}
	return rows
}

func summarizeCardinality(rows []metricCardinalityRow) (uint64, nameCountRow) {
	var active uint64
	var top nameCountRow
	for _, row := range rows {
		if row.IsTotal != 0 {
			active = row.Count
			continue
		}
		if row.Count > top.Count {
			top = nameCountRow{Name: row.Name, Count: row.Count}
		}
	}
	return active, top
}

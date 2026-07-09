package ingestion

import (
	"context"
	"fmt"
	"sort"
	"time"
)

const (
	topServiceSeries   = 5  // distinct bands in the "by service" chart; rest fold into "Other"
	topServiceRows     = 12 // rows returned for the services table
	projectionTrailing = 7  // days of trailing data the month-end projection uses
)

type Service struct {
	repo *Repository
	cfg  Config
}

func NewService(repo *Repository, cfg Config) *Service { return &Service{repo: repo, cfg: cfg} }

func dateKey(t time.Time) string { return t.UTC().Format("2006-01-02") }

// buildDateAxis returns a contiguous day-by-day axis from start to end (inclusive).
func buildDateAxis(startMs, endMs int64) []string {
	start := time.UnixMilli(startMs).UTC()
	end := time.UnixMilli(endMs).UTC()
	d := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	last := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	var out []string
	for !d.After(last) {
		out = append(out, d.Format("2006-01-02"))
		d = d.AddDate(0, 0, 1)
	}
	return out
}

func axisIndex(dates []string) map[string]int {
	idx := make(map[string]int, len(dates))
	for i, d := range dates {
		idx[d] = i
	}
	return idx
}

// fillDaily projects per-day record and byte counts onto the date axis.
func fillDaily(rows []dateCountRow, idx map[string]int, n int) (counts, bytes []uint64) {
	counts = make([]uint64, n)
	bytes = make([]uint64, n)
	for _, row := range rows {
		if i, ok := idx[dateKey(row.Day)]; ok {
			counts[i] += row.Count
			bytes[i] += row.Bytes
		}
	}
	return counts, bytes
}

func sum(values []uint64) uint64 {
	var t uint64
	for _, v := range values {
		t += v
	}
	return t
}

func daysInMonth(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func pct(part, whole uint64) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole) * 100
}

// projectMonthEnd extrapolates the month total from the mean of the last
// projectionTrailing days — a recent-pace estimate, far less noisy than a
// whole-month average early in the billing period.
func projectMonthEnd(daily []uint64, totalDays int) uint64 {
	if len(daily) == 0 {
		return 0
	}
	w := projectionTrailing
	if w > len(daily) {
		w = len(daily)
	}
	rate := float64(sum(daily[len(daily)-w:])) / float64(w)
	return uint64(rate * float64(totalDays))
}

// Summary assembles the KPI strip, by-type breakdown and metrics-pillar facts.
func (s *Service) Summary(ctx context.Context, tenantID, startMs, endMs int64) (SummaryResponse, error) {
	logs, err := s.repo.DailyLogs(ctx, tenantID, startMs, endMs)
	if err != nil {
		return SummaryResponse{}, fmt.Errorf("ingestion.Summary logs: %w", err)
	}
	spans, err := s.repo.DailySpans(ctx, tenantID, startMs, endMs)
	if err != nil {
		return SummaryResponse{}, fmt.Errorf("ingestion.Summary spans: %w", err)
	}
	metrics, err := s.repo.DailyMetricDatapoints(ctx, tenantID, startMs, endMs)
	if err != nil {
		return SummaryResponse{}, fmt.Errorf("ingestion.Summary metrics: %w", err)
	}
	activeTS, err := s.repo.ActiveTimeseries(ctx, tenantID, startMs, endMs)
	if err != nil {
		return SummaryResponse{}, fmt.Errorf("ingestion.Summary timeseries: %w", err)
	}
	topMetric, err := s.repo.TopCardinalityMetric(ctx, tenantID, startMs, endMs)
	if err != nil {
		return SummaryResponse{}, fmt.Errorf("ingestion.Summary cardinality: %w", err)
	}

	dates := buildDateAxis(startMs, endMs)
	idx := axisIndex(dates)
	n := len(dates)
	logsC, logsB := fillDaily(logs, idx, n)
	spansC, spansB := fillDaily(spans, idx, n)
	metricsC, metricsB := fillDaily(metrics, idx, n)

	logsTotal, spansTotal, metricsTotal := sum(logsC), sum(spansC), sum(metricsC)
	logsBytes, spansBytes, metricsBytes := sum(logsB), sum(spansB), sum(metricsB)
	records := logsTotal + spansTotal + metricsTotal
	bytesTotal := logsBytes + spansBytes + metricsBytes

	// Daily grand totals drive the peak day and the trailing projection.
	dailyRecords := make([]uint64, n)
	dailyBytes := make([]uint64, n)
	peak := PeakDay{}
	for i, d := range dates {
		dailyRecords[i] = logsC[i] + spansC[i] + metricsC[i]
		dailyBytes[i] = logsB[i] + spansB[i] + metricsB[i]
		if dailyRecords[i] >= peak.Records {
			peak = PeakDay{Date: d, Records: dailyRecords[i], Bytes: dailyBytes[i]}
		}
	}

	end := time.UnixMilli(endMs).UTC()
	daysElapsed := end.Day()
	totalDays := daysInMonth(end)
	var dailyAvg, dailyAvgBytes uint64
	if daysElapsed > 0 {
		dailyAvg = records / uint64(daysElapsed)
		dailyAvgBytes = bytesTotal / uint64(daysElapsed)
	}
	projRecords := projectMonthEnd(dailyRecords, totalDays)
	projBytes := projectMonthEnd(dailyBytes, totalDays)

	recCommit := s.cfg.MonthlyRecordCommitment
	byteCommit := s.cfg.MonthlyByteCommitment
	return SummaryResponse{
		Totals: SignalTotals{
			Logs: logsTotal, Spans: spansTotal, MetricDatapoints: metricsTotal, Records: records,
			LogsBytes: logsBytes, SpansBytes: spansBytes, MetricBytes: metricsBytes, Bytes: bytesTotal,
		},
		ActiveTimeseries:       activeTS,
		TopCardinalityMetric:   TopMetric{Name: topMetric.Name, Timeseries: topMetric.Count},
		DailyAverage:           dailyAvg,
		DailyAverageBytes:      dailyAvgBytes,
		Peak:                   peak,
		DaysElapsed:            daysElapsed,
		DaysInMonth:            totalDays,
		ProjectedRecords:       projRecords,
		ProjectedBytes:         projBytes,
		CommitmentRecords:      recCommit,
		CommitmentBytes:        byteCommit,
		CommitmentUsedPct:      pct(records, recCommit),
		CommitmentUsedBytesPct: pct(bytesTotal, byteCommit),
		ProjectedPct:           pct(projRecords, recCommit),
		ProjectedBytesPct:      pct(projBytes, byteCommit),
		OnPace:                 projRecords <= recCommit,
		OnPaceBytes:            projBytes <= byteCommit,
		ByType: []TypeShare{
			{Type: "logs", Label: "Logs", Records: logsTotal, Pct: pct(logsTotal, records), Bytes: logsBytes, BytesPct: pct(logsBytes, bytesTotal)},
			{Type: "spans", Label: "Spans (APM)", Records: spansTotal, Pct: pct(spansTotal, records), Bytes: spansBytes, BytesPct: pct(spansBytes, bytesTotal)},
			{Type: "metrics", Label: "Custom metrics", Records: metricsTotal, Pct: pct(metricsTotal, records), Bytes: metricsBytes, BytesPct: pct(metricsBytes, bytesTotal)},
		},
	}, nil
}

// Timeseries builds the daily stacked series, grouped by signal type or by service.
func (s *Service) Timeseries(ctx context.Context, tenantID, startMs, endMs int64, groupBy string) (TimeseriesResponse, error) {
	dates := buildDateAxis(startMs, endMs)
	idx := axisIndex(dates)

	if groupBy == "service" {
		return s.timeseriesByService(ctx, tenantID, startMs, endMs, dates, idx)
	}

	logs, err := s.repo.DailyLogs(ctx, tenantID, startMs, endMs)
	if err != nil {
		return TimeseriesResponse{}, fmt.Errorf("ingestion.Timeseries logs: %w", err)
	}
	spans, err := s.repo.DailySpans(ctx, tenantID, startMs, endMs)
	if err != nil {
		return TimeseriesResponse{}, fmt.Errorf("ingestion.Timeseries spans: %w", err)
	}
	metrics, err := s.repo.DailyMetricDatapoints(ctx, tenantID, startMs, endMs)
	if err != nil {
		return TimeseriesResponse{}, fmt.Errorf("ingestion.Timeseries metrics: %w", err)
	}

	logsC, logsB := fillDaily(logs, idx, len(dates))
	spansC, spansB := fillDaily(spans, idx, len(dates))
	metricsC, metricsB := fillDaily(metrics, idx, len(dates))
	return TimeseriesResponse{
		GroupBy: "type",
		Dates:   dates,
		Series: []TimeseriesSeries{
			{ID: "logs", Label: "Logs", Data: logsC, ByteData: logsB},
			{ID: "spans", Label: "Spans (APM)", Data: spansC, ByteData: spansB},
			{ID: "metrics", Label: "Custom metrics", Data: metricsC, ByteData: metricsB},
		},
	}, nil
}

// svcSeries accumulates a service's daily records and bytes on the date axis.
type svcSeries struct {
	counts []uint64
	bytes  []uint64
}

// accumulateByService folds per-(service, day) rows into per-service series.
func accumulateByService(rowSets [][]svcDateCountRow, idx map[string]int, n int) map[string]*svcSeries {
	perService := map[string]*svcSeries{}
	for _, rows := range rowSets {
		for _, row := range rows {
			ser := perService[row.Service]
			if ser == nil {
				ser = &svcSeries{counts: make([]uint64, n), bytes: make([]uint64, n)}
				perService[row.Service] = ser
			}
			if i, ok := idx[dateKey(row.Day)]; ok {
				ser.counts[i] += row.Count
				ser.bytes[i] += row.Bytes
			}
		}
	}
	return perService
}

func (s *Service) timeseriesByService(ctx context.Context, tenantID, startMs, endMs int64, dates []string, idx map[string]int) (TimeseriesResponse, error) {
	logs, err := s.repo.DailyLogsByService(ctx, tenantID, startMs, endMs)
	if err != nil {
		return TimeseriesResponse{}, fmt.Errorf("ingestion.Timeseries svc logs: %w", err)
	}
	spans, err := s.repo.DailySpansByService(ctx, tenantID, startMs, endMs)
	if err != nil {
		return TimeseriesResponse{}, fmt.Errorf("ingestion.Timeseries svc spans: %w", err)
	}

	perService := accumulateByService([][]svcDateCountRow{logs, spans}, idx, len(dates))
	ranked := rankByTotal(perService)
	series := make([]TimeseriesSeries, 0, topServiceSeries+1)
	otherC := make([]uint64, len(dates))
	otherB := make([]uint64, len(dates))
	for rank, name := range ranked {
		ser := perService[name]
		if rank < topServiceSeries {
			series = append(series, TimeseriesSeries{ID: name, Label: name, Data: ser.counts, ByteData: ser.bytes})
			continue
		}
		for i := range otherC {
			otherC[i] += ser.counts[i]
			otherB[i] += ser.bytes[i]
		}
	}
	if len(ranked) > topServiceSeries {
		series = append(series, TimeseriesSeries{ID: "other", Label: "Other services", Data: otherC, ByteData: otherB})
	}

	return TimeseriesResponse{GroupBy: "service", Dates: dates, Series: series}, nil
}

// rankByTotal returns service names ordered by descending total record volume.
func rankByTotal(perService map[string]*svcSeries) []string {
	names := make([]string, 0, len(perService))
	for name := range perService {
		names = append(names, name)
	}
	sort.Slice(names, func(a, b int) bool {
		ta, tb := sum(perService[names[a]].counts), sum(perService[names[b]].counts)
		if ta == tb {
			return names[a] < names[b]
		}
		return ta > tb
	})
	return names
}

// serviceAgg is a service's period totals for the top-services table. Records
// and bytes are logs+spans; timeseries is metric cardinality (a separate axis).
type serviceAgg struct {
	env        string
	logs       uint64
	spans      uint64
	timeseries uint64
	logsBytes  uint64
	spansBytes uint64
}

func (a *serviceAgg) records() uint64 { return a.logs + a.spans }
func (a *serviceAgg) bytes() uint64   { return a.logsBytes + a.spansBytes }

// aggregateServices folds the per-signal totals into one map keyed by service.
func aggregateServices(logTotals, spanTotals, tsTotals []svcCountRow) map[string]*serviceAgg {
	services := map[string]*serviceAgg{}
	get := func(name string) *serviceAgg {
		a := services[name]
		if a == nil {
			a = &serviceAgg{}
			services[name] = a
		}
		return a
	}
	for _, row := range logTotals {
		a := get(row.Service)
		a.logs, a.logsBytes, a.env = row.Count, row.Bytes, row.Env
	}
	for _, row := range spanTotals {
		a := get(row.Service)
		a.spans, a.spansBytes = row.Count, row.Bytes
		if a.env == "" {
			a.env = row.Env
		}
	}
	for _, row := range tsTotals {
		get(row.Service).timeseries = row.Count
	}
	return services
}

// priorRecordTotals sums the prior window's records per service for the delta.
func priorRecordTotals(priorLogs, priorSpans []svcCountRow) map[string]uint64 {
	totals := map[string]uint64{}
	for _, row := range priorLogs {
		totals[row.Service] += row.Count
	}
	for _, row := range priorSpans {
		totals[row.Service] += row.Count
	}
	return totals
}

// Services builds the top-ingesting-services table, including a prior-period delta.
func (s *Service) Services(ctx context.Context, tenantID, startMs, endMs int64) (ServicesResponse, error) {
	logTotals, err := s.repo.ServiceLogTotals(ctx, tenantID, startMs, endMs)
	if err != nil {
		return ServicesResponse{}, fmt.Errorf("ingestion.Services logs: %w", err)
	}
	spanTotals, err := s.repo.ServiceSpanTotals(ctx, tenantID, startMs, endMs)
	if err != nil {
		return ServicesResponse{}, fmt.Errorf("ingestion.Services spans: %w", err)
	}
	tsTotals, err := s.repo.ServiceTimeseries(ctx, tenantID, startMs, endMs)
	if err != nil {
		return ServicesResponse{}, fmt.Errorf("ingestion.Services timeseries: %w", err)
	}

	// Prior equal-length window for the delta column.
	span := endMs - startMs
	priorLogs, err := s.repo.ServiceLogTotals(ctx, tenantID, startMs-span, startMs)
	if err != nil {
		return ServicesResponse{}, fmt.Errorf("ingestion.Services prior logs: %w", err)
	}
	priorSpans, err := s.repo.ServiceSpanTotals(ctx, tenantID, startMs-span, startMs)
	if err != nil {
		return ServicesResponse{}, fmt.Errorf("ingestion.Services prior spans: %w", err)
	}

	dates := buildDateAxis(startMs, endMs)
	idx := axisIndex(dates)
	dailyLogs, err := s.repo.DailyLogsByService(ctx, tenantID, startMs, endMs)
	if err != nil {
		return ServicesResponse{}, fmt.Errorf("ingestion.Services daily logs: %w", err)
	}
	dailySpans, err := s.repo.DailySpansByService(ctx, tenantID, startMs, endMs)
	if err != nil {
		return ServicesResponse{}, fmt.Errorf("ingestion.Services daily spans: %w", err)
	}

	services := aggregateServices(logTotals, spanTotals, tsTotals)
	priorTotals := priorRecordTotals(priorLogs, priorSpans)
	spark := accumulateByService([][]svcDateCountRow{dailyLogs, dailySpans}, idx, len(dates))

	return buildServicesResponse(services, priorTotals, spark, len(dates)), nil
}

// buildServicesResponse ranks services by record volume, keeps the top rows and
// computes each row's share, delta and sparklines.
func buildServicesResponse(services map[string]*serviceAgg, prior map[string]uint64, spark map[string]*svcSeries, n int) ServicesResponse {
	var grandTotal, grandBytes uint64
	names := make([]string, 0, len(services))
	for name, a := range services {
		// Skip unattributable telemetry (e.g. servicegraph edge metrics carry
		// no service.name); a blank 0-record row is not a real service.
		if name == "" {
			continue
		}
		grandTotal += a.records()
		grandBytes += a.bytes()
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		ti, tj := services[names[i]].records(), services[names[j]].records()
		if ti == tj {
			return names[i] < names[j]
		}
		return ti > tj
	})

	limit := topServiceRows
	if limit > len(names) {
		limit = len(names)
	}
	rows := make([]ServiceRow, 0, limit)
	var topSum, topByteSum uint64
	for _, name := range names[:limit] {
		a := services[name]
		total, bytes := a.records(), a.bytes()
		topSum += total
		topByteSum += bytes
		delta := 0.0
		if p := prior[name]; p > 0 {
			delta = (float64(total) - float64(p)) / float64(p) * 100
		}
		// Zero-fill sparks so JSON is never null (nil slices marshal to null).
		var spC, spB []uint64
		if ser := spark[name]; ser != nil {
			spC, spB = ser.counts, ser.bytes
		} else {
			spC, spB = make([]uint64, n), make([]uint64, n)
		}
		rows = append(rows, ServiceRow{
			Name:       name,
			Env:        a.env,
			Logs:       a.logs,
			Spans:      a.spans,
			Timeseries: a.timeseries,
			Total:      total,
			Bytes:      bytes,
			Pct:        pct(total, grandTotal),
			BytesPct:   pct(bytes, grandBytes),
			DeltaPct:   delta,
			Spark:      spC,
			ByteSpark:  spB,
		})
	}

	return ServicesResponse{
		Services:         rows,
		TotalServices:    len(names),
		TopSharePct:      pct(topSum, grandTotal),
		TopShareBytesPct: pct(topByteSum, grandBytes),
	}
}

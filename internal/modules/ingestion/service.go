package ingestion

import (
	"context"
	"fmt"
	"sort"
	"time"
)

const (
	topServiceSeries = 5  // distinct bands in the "by service" chart; remainder folds into "Other"
	topServiceRows   = 12 // rows returned for the services table
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

// fillDaily projects per-day rows onto the date axis.
func fillDaily(rows []dateCountRow, idx map[string]int, n int) []uint64 {
	out := make([]uint64, n)
	for _, row := range rows {
		if i, ok := idx[dateKey(row.Day)]; ok {
			out[i] += row.Count
		}
	}
	return out
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

// Summary assembles the KPI strip, by-type breakdown and metrics-pillar facts.
func (s *Service) Summary(ctx context.Context, teamID, startMs, endMs int64) (SummaryResponse, error) {
	logs, err := s.repo.DailyLogs(ctx, teamID, startMs, endMs)
	if err != nil {
		return SummaryResponse{}, fmt.Errorf("ingestion.Summary logs: %w", err)
	}
	spans, err := s.repo.DailySpans(ctx, teamID, startMs, endMs)
	if err != nil {
		return SummaryResponse{}, fmt.Errorf("ingestion.Summary spans: %w", err)
	}
	metrics, err := s.repo.DailyMetricDatapoints(ctx, teamID, startMs, endMs)
	if err != nil {
		return SummaryResponse{}, fmt.Errorf("ingestion.Summary metrics: %w", err)
	}
	activeTS, err := s.repo.ActiveTimeseries(ctx, teamID, startMs, endMs)
	if err != nil {
		return SummaryResponse{}, fmt.Errorf("ingestion.Summary timeseries: %w", err)
	}
	topMetric, err := s.repo.TopCardinalityMetric(ctx, teamID, startMs, endMs)
	if err != nil {
		return SummaryResponse{}, fmt.Errorf("ingestion.Summary cardinality: %w", err)
	}

	dates := buildDateAxis(startMs, endMs)
	idx := axisIndex(dates)
	logsDaily := fillDaily(logs, idx, len(dates))
	spansDaily := fillDaily(spans, idx, len(dates))
	metricsDaily := fillDaily(metrics, idx, len(dates))

	logsTotal, spansTotal, metricsTotal := sum(logsDaily), sum(spansDaily), sum(metricsDaily)
	records := logsTotal + spansTotal + metricsTotal

	peak := PeakDay{}
	for i, d := range dates {
		day := logsDaily[i] + spansDaily[i] + metricsDaily[i]
		if day >= peak.Records {
			peak = PeakDay{Date: d, Records: day}
		}
	}

	end := time.UnixMilli(endMs).UTC()
	daysElapsed := end.Day()
	totalDays := daysInMonth(end)
	var dailyAvg, projected uint64
	if daysElapsed > 0 {
		dailyAvg = records / uint64(daysElapsed)
		projected = dailyAvg * uint64(totalDays)
	}

	commitment := s.cfg.MonthlyRecordCommitment
	return SummaryResponse{
		Totals:               SignalTotals{Logs: logsTotal, Spans: spansTotal, MetricDatapoints: metricsTotal, Records: records},
		ActiveTimeseries:     activeTS,
		TopCardinalityMetric: TopMetric{Name: topMetric.Name, Timeseries: topMetric.Count},
		DailyAverage:         dailyAvg,
		Peak:                 peak,
		DaysElapsed:          daysElapsed,
		DaysInMonth:          totalDays,
		ProjectedRecords:     projected,
		CommitmentRecords:    commitment,
		CommitmentUsedPct:    pct(records, commitment),
		ProjectedPct:         pct(projected, commitment),
		OnPace:               projected <= commitment,
		ByType: []TypeShare{
			{Type: "logs", Label: "Logs", Records: logsTotal, Pct: pct(logsTotal, records)},
			{Type: "spans", Label: "Spans (APM)", Records: spansTotal, Pct: pct(spansTotal, records)},
			{Type: "metrics", Label: "Custom metrics", Records: metricsTotal, Pct: pct(metricsTotal, records)},
		},
	}, nil
}

// Timeseries builds the daily stacked series, grouped by signal type or by service.
func (s *Service) Timeseries(ctx context.Context, teamID, startMs, endMs int64, groupBy string) (TimeseriesResponse, error) {
	dates := buildDateAxis(startMs, endMs)
	idx := axisIndex(dates)

	if groupBy == "service" {
		return s.timeseriesByService(ctx, teamID, startMs, endMs, dates, idx)
	}

	logs, err := s.repo.DailyLogs(ctx, teamID, startMs, endMs)
	if err != nil {
		return TimeseriesResponse{}, fmt.Errorf("ingestion.Timeseries logs: %w", err)
	}
	spans, err := s.repo.DailySpans(ctx, teamID, startMs, endMs)
	if err != nil {
		return TimeseriesResponse{}, fmt.Errorf("ingestion.Timeseries spans: %w", err)
	}
	metrics, err := s.repo.DailyMetricDatapoints(ctx, teamID, startMs, endMs)
	if err != nil {
		return TimeseriesResponse{}, fmt.Errorf("ingestion.Timeseries metrics: %w", err)
	}

	return TimeseriesResponse{
		GroupBy: "type",
		Dates:   dates,
		Series: []TimeseriesSeries{
			{ID: "logs", Label: "Logs", Data: fillDaily(logs, idx, len(dates))},
			{ID: "spans", Label: "Spans (APM)", Data: fillDaily(spans, idx, len(dates))},
			{ID: "metrics", Label: "Custom metrics", Data: fillDaily(metrics, idx, len(dates))},
		},
	}, nil
}

func (s *Service) timeseriesByService(ctx context.Context, teamID, startMs, endMs int64, dates []string, idx map[string]int) (TimeseriesResponse, error) {
	logs, err := s.repo.DailyLogsByService(ctx, teamID, startMs, endMs)
	if err != nil {
		return TimeseriesResponse{}, fmt.Errorf("ingestion.Timeseries svc logs: %w", err)
	}
	spans, err := s.repo.DailySpansByService(ctx, teamID, startMs, endMs)
	if err != nil {
		return TimeseriesResponse{}, fmt.Errorf("ingestion.Timeseries svc spans: %w", err)
	}

	perService := map[string][]uint64{}
	put := func(rows []svcDateCountRow) {
		for _, row := range rows {
			arr, ok := perService[row.Service]
			if !ok {
				arr = make([]uint64, len(dates))
				perService[row.Service] = arr
			}
			if i, ok := idx[dateKey(row.Day)]; ok {
				arr[i] += row.Count
			}
		}
	}
	put(logs)
	put(spans)

	ranked := rankByTotal(perService)
	series := make([]TimeseriesSeries, 0, topServiceSeries+1)
	other := make([]uint64, len(dates))
	for rank, name := range ranked {
		if rank < topServiceSeries {
			series = append(series, TimeseriesSeries{ID: name, Label: name, Data: perService[name]})
			continue
		}
		for i, v := range perService[name] {
			other[i] += v
		}
	}
	if len(ranked) > topServiceSeries {
		series = append(series, TimeseriesSeries{ID: "other", Label: "Other services", Data: other})
	}

	return TimeseriesResponse{GroupBy: "service", Dates: dates, Series: series}, nil
}

// rankByTotal returns service names ordered by descending total volume.
func rankByTotal(perService map[string][]uint64) []string {
	names := make([]string, 0, len(perService))
	for name := range perService {
		names = append(names, name)
	}
	sort.Slice(names, func(a, b int) bool {
		ta, tb := sum(perService[names[a]]), sum(perService[names[b]])
		if ta == tb {
			return names[a] < names[b]
		}
		return ta > tb
	})
	return names
}

// Services builds the top-ingesting-services table, including a prior-period delta.
func (s *Service) Services(ctx context.Context, teamID, startMs, endMs int64) (ServicesResponse, error) {
	logTotals, err := s.repo.ServiceLogTotals(ctx, teamID, startMs, endMs)
	if err != nil {
		return ServicesResponse{}, fmt.Errorf("ingestion.Services logs: %w", err)
	}
	spanTotals, err := s.repo.ServiceSpanTotals(ctx, teamID, startMs, endMs)
	if err != nil {
		return ServicesResponse{}, fmt.Errorf("ingestion.Services spans: %w", err)
	}
	tsTotals, err := s.repo.ServiceTimeseries(ctx, teamID, startMs, endMs)
	if err != nil {
		return ServicesResponse{}, fmt.Errorf("ingestion.Services timeseries: %w", err)
	}

	// Prior equal-length window for the delta column.
	span := endMs - startMs
	priorLogs, err := s.repo.ServiceLogTotals(ctx, teamID, startMs-span, startMs)
	if err != nil {
		return ServicesResponse{}, fmt.Errorf("ingestion.Services prior logs: %w", err)
	}
	priorSpans, err := s.repo.ServiceSpanTotals(ctx, teamID, startMs-span, startMs)
	if err != nil {
		return ServicesResponse{}, fmt.Errorf("ingestion.Services prior spans: %w", err)
	}

	dates := buildDateAxis(startMs, endMs)
	idx := axisIndex(dates)
	dailyLogs, err := s.repo.DailyLogsByService(ctx, teamID, startMs, endMs)
	if err != nil {
		return ServicesResponse{}, fmt.Errorf("ingestion.Services daily logs: %w", err)
	}
	dailySpans, err := s.repo.DailySpansByService(ctx, teamID, startMs, endMs)
	if err != nil {
		return ServicesResponse{}, fmt.Errorf("ingestion.Services daily spans: %w", err)
	}

	type agg struct {
		env        string
		logs       uint64
		spans      uint64
		timeseries uint64
	}
	services := map[string]*agg{}
	get := func(name string) *agg {
		a, ok := services[name]
		if !ok {
			a = &agg{}
			services[name] = a
		}
		return a
	}
	for _, row := range logTotals {
		a := get(row.Service)
		a.logs = row.Count
		a.env = row.Env
	}
	for _, row := range spanTotals {
		get(row.Service).spans = row.Count
	}
	for _, row := range tsTotals {
		get(row.Service).timeseries = row.Count
	}

	priorTotals := map[string]uint64{}
	for _, row := range priorLogs {
		priorTotals[row.Service] += row.Count
	}
	for _, row := range priorSpans {
		priorTotals[row.Service] += row.Count
	}

	spark := map[string][]uint64{}
	putSpark := func(rows []svcDateCountRow) {
		for _, row := range rows {
			arr, ok := spark[row.Service]
			if !ok {
				arr = make([]uint64, len(dates))
				spark[row.Service] = arr
			}
			if i, ok := idx[dateKey(row.Day)]; ok {
				arr[i] += row.Count
			}
		}
	}
	putSpark(dailyLogs)
	putSpark(dailySpans)

	var grandTotal uint64
	names := make([]string, 0, len(services))
	for name, a := range services {
		grandTotal += a.logs + a.spans
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		ti := services[names[i]].logs + services[names[i]].spans
		tj := services[names[j]].logs + services[names[j]].spans
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
	var topSum uint64
	for _, name := range names[:limit] {
		a := services[name]
		total := a.logs + a.spans
		topSum += total
		delta := 0.0
		if prior := priorTotals[name]; prior > 0 {
			delta = (float64(total) - float64(prior)) / float64(prior) * 100
		}
		// Metrics-only services have no daily log/span rows; emit a zero-filled
		// spark so the JSON is never null (nil slices marshal to null).
		sp := spark[name]
		if sp == nil {
			sp = make([]uint64, len(dates))
		}
		rows = append(rows, ServiceRow{
			Name:       name,
			Env:        a.env,
			Logs:       a.logs,
			Spans:      a.spans,
			Timeseries: a.timeseries,
			Total:      total,
			Pct:        pct(total, grandTotal),
			DeltaPct:   delta,
			Spark:      sp,
		})
	}

	return ServicesResponse{
		Services:      rows,
		TotalServices: len(names),
		TopSharePct:   pct(topSum, grandTotal),
	}, nil
}

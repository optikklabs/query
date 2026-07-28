package ingestion

import (
	"time"
)

const (
	topServiceSeries = 5
	topServiceRows   = 12
)

type Service struct {
	repo *Repository
	cfg  Config
}

func NewService(repo *Repository, cfg Config) *Service { return &Service{repo: repo, cfg: cfg} }

func dateKey(t time.Time) string { return t.UTC().Format("2006-01-02") }

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

func (s *Service) summaryFromUsage(
	logs, spans, metrics []dateCountRow,
	activeTS uint64,
	topMetric nameCountRow,
	startMs, endMs int64,
) SummaryResponse {
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

	peak := PeakDay{}
	for i, d := range dates {
		dayRecords := logsC[i] + spansC[i] + metricsC[i]
		dayBytes := logsB[i] + spansB[i] + metricsB[i]
		if dayRecords >= peak.Records {
			peak = PeakDay{Date: d, Records: dayRecords, Bytes: dayBytes}
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
		CommitmentRecords:      recCommit,
		CommitmentBytes:        byteCommit,
		CommitmentUsedPct:      pct(records, recCommit),
		CommitmentUsedBytesPct: pct(bytesTotal, byteCommit),
		ByType: []TypeShare{
			{Type: "logs", Label: "Logs", Records: logsTotal, Pct: pct(logsTotal, records), Bytes: logsBytes, BytesPct: pct(logsBytes, bytesTotal)},
			{Type: "spans", Label: "Spans (APM)", Records: spansTotal, Pct: pct(spansTotal, records), Bytes: spansBytes, BytesPct: pct(spansBytes, bytesTotal)},
			{Type: "metrics", Label: "Custom metrics", Records: metricsTotal, Pct: pct(metricsTotal, records), Bytes: metricsBytes, BytesPct: pct(metricsBytes, bytesTotal)},
		},
	}
}

func (s *Service) costFromUsage(
	logs, spans, metrics []dateCountRow,
	startMs, endMs int64,
) CostResponse {
	var logsBytes, spansBytes, metricDPs uint64
	for _, row := range logs {
		logsBytes += row.Bytes
	}
	for _, row := range spans {
		spansBytes += row.Bytes
	}
	for _, row := range metrics {
		metricDPs += row.Count
	}

	end := time.UnixMilli(endMs).UTC()
	u := usageQuantities{
		logsBytes:   logsBytes,
		spansBytes:  spansBytes,
		metricDPs:   metricDPs,
		windowMin:   float64(endMs-startMs) / float64(time.Minute/time.Millisecond),
		daysElapsed: end.Day(),
		daysInMonth: daysInMonth(end),
	}
	return estimateCost(u, s.cfg.Rates())
}

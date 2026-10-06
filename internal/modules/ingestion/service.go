package ingestion

import (
	"time"

	"github.com/optikklabs/query/internal/shared/metrics"
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

const dateLayout = time.DateOnly

// dateAxis is the list of UTC days a report covers, with each day's index.
type dateAxis struct {
	dates []string
	index map[string]int
}

func newDateAxis(startMs, endMs int64) dateAxis {
	day := time.UnixMilli(startMs).UTC().Truncate(24 * time.Hour)
	last := time.UnixMilli(endMs).UTC().Truncate(24 * time.Hour)
	axis := dateAxis{index: map[string]int{}}
	for ; !day.After(last); day = day.AddDate(0, 0, 1) {
		axis.index[day.Format(dateLayout)] = len(axis.dates)
		axis.dates = append(axis.dates, day.Format(dateLayout))
	}
	return axis
}

// position returns t's index on the axis.
func (a dateAxis) position(t time.Time) (int, bool) {
	i, ok := a.index[t.UTC().Format(dateLayout)]
	return i, ok
}

// fill sums rows into per-day count and byte series aligned to the axis.
func (a dateAxis) fill(rows []dateCountRow) (counts, bytes []uint64) {
	counts = make([]uint64, len(a.dates))
	bytes = make([]uint64, len(a.dates))
	for _, row := range rows {
		if i, ok := a.position(row.Day); ok {
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

func (s *Service) summaryFromUsage(
	axis dateAxis,
	logs, spans, metricRows []dateCountRow,
	activeTS uint64,
	topMetric nameCountRow,
	endMs int64,
) SummaryResponse {
	logsC, logsB := axis.fill(logs)
	spansC, spansB := axis.fill(spans)
	metricsC, metricsB := axis.fill(metricRows)

	logsTotal, spansTotal, metricsTotal := sum(logsC), sum(spansC), sum(metricsC)
	logsBytes, spansBytes, metricsBytes := sum(logsB), sum(spansB), sum(metricsB)
	records := logsTotal + spansTotal + metricsTotal
	bytesTotal := logsBytes + spansBytes + metricsBytes

	peak := PeakDay{}
	for i, d := range axis.dates {
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
		CommitmentUsedPct:      metrics.Percentage(records, recCommit),
		CommitmentUsedBytesPct: metrics.Percentage(bytesTotal, byteCommit),
		ByType: []TypeShare{
			{Type: "logs", Label: "Logs", Records: logsTotal, Pct: metrics.Percentage(logsTotal, records), Bytes: logsBytes, BytesPct: metrics.Percentage(logsBytes, bytesTotal)},
			{Type: "spans", Label: "Spans (APM)", Records: spansTotal, Pct: metrics.Percentage(spansTotal, records), Bytes: spansBytes, BytesPct: metrics.Percentage(spansBytes, bytesTotal)},
			{Type: "metrics", Label: "Custom metrics", Records: metricsTotal, Pct: metrics.Percentage(metricsTotal, records), Bytes: metricsBytes, BytesPct: metrics.Percentage(metricsBytes, bytesTotal)},
		},
	}
}

func (s *Service) costFromUsage(
	logs, spans, metricRows []dateCountRow,
	endMs int64,
) CostResponse {
	var logsBytes, spansBytes, metricSamples uint64
	for _, row := range logs {
		logsBytes += row.Bytes
	}
	for _, row := range spans {
		spansBytes += row.Bytes
	}
	for _, row := range metricRows {
		metricSamples += row.Count
	}

	end := time.UnixMilli(endMs).UTC()
	u := usageQuantities{
		logsBytes:     logsBytes,
		spansBytes:    spansBytes,
		metricSamples: metricSamples,
		daysElapsed:   end.Day(),
		daysInMonth:   daysInMonth(end),
	}
	return estimateCost(u, s.cfg.Rates())
}

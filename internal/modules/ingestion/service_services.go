package ingestion

import (
	"cmp"
	"slices"
	"strings"

	"github.com/optikklabs/query/internal/shared/metrics"
)

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

func servicesFromUsage(axis dateAxis, usage serviceUsageSets, tsTotals []svcCountRow) ServicesResponse {
	services := aggregateServices(usage.logTotals, usage.spanTotals, tsTotals)
	priorTotals := priorRecordTotals(usage.priorLogs, usage.priorSpans)
	spark := accumulateByService(axis, usage.dailyLogs, usage.dailySpans)
	return buildServicesResponse(services, priorTotals, spark, len(axis.dates))
}

func buildServicesResponse(services map[string]*serviceAgg, prior map[string]uint64, spark map[string]*svcSeries, n int) ServicesResponse {
	names, grandTotal, grandBytes := rankedServices(services)
	limit := min(topServiceRows, len(names))
	rows := make([]ServiceRow, 0, limit)
	var topSum, topByteSum uint64
	for _, name := range names[:limit] {
		row := buildServiceRow(name, services[name], prior[name], spark[name], n, grandTotal, grandBytes)
		topSum += row.Total
		topByteSum += row.Bytes
		rows = append(rows, row)
	}
	return ServicesResponse{
		Services: rows, TotalServices: len(names),
		TopSharePct: metrics.Percentage(topSum, grandTotal), TopShareBytesPct: metrics.Percentage(topByteSum, grandBytes),
	}
}

func rankedServices(services map[string]*serviceAgg) ([]string, uint64, uint64) {
	var totalRecords, totalBytes uint64
	names := make([]string, 0, len(services))
	for name, a := range services {
		if name == "" {
			continue
		}
		totalRecords += a.records()
		totalBytes += a.bytes()
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int {
		return cmp.Or(cmp.Compare(services[b].records(), services[a].records()), strings.Compare(a, b))
	})
	return names, totalRecords, totalBytes
}

func buildServiceRow(name string, agg *serviceAgg, prior uint64, spark *svcSeries, n int, totalRecords, totalBytes uint64) ServiceRow {
	total, bytes := agg.records(), agg.bytes()
	delta := 0.0
	if prior > 0 {
		delta = (float64(total) - float64(prior)) / float64(prior) * 100
	}
	counts, byteSpark := make([]uint64, n), make([]uint64, n)
	if spark != nil {
		counts, byteSpark = spark.counts, spark.bytes
	}
	return ServiceRow{
		Name: name, Env: agg.env, Logs: agg.logs, Spans: agg.spans, Timeseries: agg.timeseries,
		Total: total, Bytes: bytes, Pct: metrics.Percentage(total, totalRecords), BytesPct: metrics.Percentage(bytes, totalBytes),
		DeltaPct: delta, Spark: counts, ByteSpark: byteSpark,
	}
}

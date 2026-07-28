package ingestion

import (
	"sort"
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

func servicesFromUsage(
	logTotals, spanTotals, tsTotals, priorLogs, priorSpans []svcCountRow,
	dailyLogs, dailySpans []svcDateCountRow,
	startMs, endMs int64,
) ServicesResponse {
	dates := buildDateAxis(startMs, endMs)
	idx := axisIndex(dates)
	services := aggregateServices(logTotals, spanTotals, tsTotals)
	priorTotals := priorRecordTotals(priorLogs, priorSpans)
	spark := accumulateByService([][]svcDateCountRow{dailyLogs, dailySpans}, idx, len(dates))

	return buildServicesResponse(services, priorTotals, spark, len(dates))
}

func buildServicesResponse(services map[string]*serviceAgg, prior map[string]uint64, spark map[string]*svcSeries, n int) ServicesResponse {
	var grandTotal, grandBytes uint64
	names := make([]string, 0, len(services))
	for name, a := range services {

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

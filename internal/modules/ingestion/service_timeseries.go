package ingestion

import (
	"sort"
)

func timeseriesByType(
	logs, spans, metrics []dateCountRow,
	dates []string,
	idx map[string]int,
) TimeseriesResponse {
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
	}
}

type svcSeries struct {
	counts []uint64
	bytes  []uint64
}

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

func timeseriesByServiceRows(
	logs, spans []svcDateCountRow,
	dates []string,
	idx map[string]int,
) TimeseriesResponse {
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

	return TimeseriesResponse{GroupBy: "service", Dates: dates, Series: series}
}

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

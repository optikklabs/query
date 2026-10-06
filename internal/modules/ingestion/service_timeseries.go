package ingestion

import (
	"cmp"
	"maps"
	"slices"
	"strings"
)

func timeseriesByType(axis dateAxis, logs, spans, metrics []dateCountRow) TimeseriesResponse {
	logsC, logsB := axis.fill(logs)
	spansC, spansB := axis.fill(spans)
	metricsC, metricsB := axis.fill(metrics)
	return TimeseriesResponse{
		GroupBy: "type",
		Dates:   axis.dates,
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

func accumulateByService(axis dateAxis, rowSets ...[]svcDateCountRow) map[string]*svcSeries {
	n := len(axis.dates)
	perService := map[string]*svcSeries{}
	for _, rows := range rowSets {
		for _, row := range rows {
			ser := perService[row.Service]
			if ser == nil {
				ser = &svcSeries{counts: make([]uint64, n), bytes: make([]uint64, n)}
				perService[row.Service] = ser
			}
			if i, ok := axis.position(row.Day); ok {
				ser.counts[i] += row.Count
				ser.bytes[i] += row.Bytes
			}
		}
	}
	return perService
}

func timeseriesByServiceRows(axis dateAxis, logs, spans []svcDateCountRow) TimeseriesResponse {
	perService := accumulateByService(axis, logs, spans)
	ranked := rankByTotal(perService)
	series := make([]TimeseriesSeries, 0, topServiceSeries+1)
	otherC := make([]uint64, len(axis.dates))
	otherB := make([]uint64, len(axis.dates))
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

	return TimeseriesResponse{GroupBy: "service", Dates: axis.dates, Series: series}
}

// rankByTotal orders services by record volume, busiest first.
func rankByTotal(perService map[string]*svcSeries) []string {
	return slices.SortedFunc(maps.Keys(perService), func(a, b string) int {
		return cmp.Or(cmp.Compare(sum(perService[b].counts), sum(perService[a].counts)), strings.Compare(a, b))
	})
}

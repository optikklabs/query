package ingestion

const bytesPerGB = 1_000_000_000.0

type Rates struct {
	Currency        string
	PerGBLogsTraces float64
	PerDPMMetrics   float64
}

type CostLine struct {
	Category string  `json:"category"`
	Unit     string  `json:"unit"`
	Quantity float64 `json:"quantity"`
	Rate     float64 `json:"rate"`
	Cost     float64 `json:"cost"`
}

type CostResponse struct {
	Currency    string     `json:"currency"`
	Lines       []CostLine `json:"lines"`
	CurrentCost float64    `json:"currentCost"`
	DaysElapsed int        `json:"daysElapsed"`
	DaysInMonth int        `json:"daysInMonth"`
}

type usageQuantities struct {
	logsBytes   uint64
	spansBytes  uint64
	metricDPs   uint64
	windowMin   float64
	daysElapsed int
	daysInMonth int
}

func estimateCost(u usageQuantities, r Rates) CostResponse {
	var dpm float64
	if u.windowMin > 0 {
		dpm = float64(u.metricDPs) / u.windowMin
	}
	metricsCost := dpm * r.PerDPMMetrics
	lines := []CostLine{
		volumeLine("Logs", gigabytes(u.logsBytes), r.PerGBLogsTraces),
		volumeLine("Traces", gigabytes(u.spansBytes), r.PerGBLogsTraces),
		{Category: "Metrics", Unit: "DPM", Quantity: dpm, Rate: r.PerDPMMetrics,
			Cost: metricsCost},
	}
	var current float64
	for _, l := range lines {
		current += l.Cost
	}
	return CostResponse{
		Currency:    r.Currency,
		Lines:       lines,
		CurrentCost: current,
		DaysElapsed: u.daysElapsed,
		DaysInMonth: u.daysInMonth,
	}
}

func gigabytes(b uint64) float64 { return float64(b) / bytesPerGB }

func volumeLine(category string, gb, rate float64) CostLine {
	return CostLine{
		Category: category,
		Unit:     "GB",
		Quantity: gb,
		Rate:     rate,
		Cost:     gb * rate,
	}
}

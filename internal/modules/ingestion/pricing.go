package ingestion

// bytesPerGB uses the decimal GB (10^9), the standard billing convention.
const bytesPerGB = 1_000_000_000.0

// Rates are the per-unit prices used to estimate a tenant's bill. Logs and
// traces bill by ingested volume (per GB); metrics bill by DPM (data points per
// minute), the industry-standard rate meter for time series.
type Rates struct {
	Currency        string
	PerGBLogsTraces float64
	PerDPMMetrics   float64
}

// CostLine is one billable meter: quantity in its unit at a rate, the
// month-to-date cost and the projected full-month cost.
type CostLine struct {
	Category      string  `json:"category"`
	Unit          string  `json:"unit"`
	Quantity      float64 `json:"quantity"`
	Rate          float64 `json:"rate"`
	Cost          float64 `json:"cost"`
	ProjectedCost float64 `json:"projectedCost"`
}

// CostResponse is the tenant's estimated bill for the billing period.
type CostResponse struct {
	Currency             string     `json:"currency"`
	Lines                []CostLine `json:"lines"`
	CurrentCost          float64    `json:"currentCost"`
	ProjectedMonthlyCost float64    `json:"projectedMonthlyCost"`
	DaysElapsed          int        `json:"daysElapsed"`
	DaysInMonth          int        `json:"daysInMonth"`
}

// usageQuantities are the raw meter inputs the estimate is derived from.
type usageQuantities struct {
	logsBytes   uint64
	spansBytes  uint64
	metricDPs   uint64
	windowMin   float64 // minutes of data in the period, denominator for DPM
	daysElapsed int
	daysInMonth int
}

// estimateCost turns usage quantities into a billing estimate. Volume meters
// (GB) accumulate, so they project linearly by elapsed days; the DPM meter is a
// rate, so its cost is already a monthly figure and projects flat.
func estimateCost(u usageQuantities, r Rates) CostResponse {
	proj := 1.0
	if u.daysElapsed > 0 {
		proj = float64(u.daysInMonth) / float64(u.daysElapsed)
	}
	var dpm float64
	if u.windowMin > 0 {
		dpm = float64(u.metricDPs) / u.windowMin
	}
	metricsCost := dpm * r.PerDPMMetrics
	lines := []CostLine{
		volumeLine("Logs", gigabytes(u.logsBytes), r.PerGBLogsTraces, proj),
		volumeLine("Traces", gigabytes(u.spansBytes), r.PerGBLogsTraces, proj),
		{Category: "Metrics", Unit: "DPM", Quantity: dpm, Rate: r.PerDPMMetrics,
			Cost: metricsCost, ProjectedCost: metricsCost},
	}
	var current, projected float64
	for _, l := range lines {
		current += l.Cost
		projected += l.ProjectedCost
	}
	return CostResponse{
		Currency:             r.Currency,
		Lines:                lines,
		CurrentCost:          current,
		ProjectedMonthlyCost: projected,
		DaysElapsed:          u.daysElapsed,
		DaysInMonth:          u.daysInMonth,
	}
}

func gigabytes(b uint64) float64 { return float64(b) / bytesPerGB }

// volumeLine bills a GB-metered category: MTD cost now, linear month projection.
func volumeLine(category string, gb, rate, proj float64) CostLine {
	return CostLine{
		Category:      category,
		Unit:          "GB",
		Quantity:      gb,
		Rate:          rate,
		Cost:          gb * rate,
		ProjectedCost: gb * proj * rate,
	}
}

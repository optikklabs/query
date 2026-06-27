package ingestion

// Config holds operator-tunable settings for the ingestion usage view.
// MonthlyRecordCommitment is the contracted ingestion budget expressed in
// records/month (logs + spans + metric datapoints); it has no source in the
// telemetry store, so it lives here as a configurable constant.
type Config struct {
	Enabled                 bool
	MonthlyRecordCommitment uint64
}

func DefaultConfig() Config {
	return Config{Enabled: true, MonthlyRecordCommitment: 5_000_000_000}
}

// SignalTotals carries the period record counts per telemetry signal.
type SignalTotals struct {
	Logs             uint64 `json:"logs"`
	Spans            uint64 `json:"spans"`
	MetricDatapoints uint64 `json:"metricDatapoints"`
	Records          uint64 `json:"records"`
}

// TypeShare is one telemetry type's contribution to the period total.
type TypeShare struct {
	Type    string  `json:"type"`
	Label   string  `json:"label"`
	Records uint64  `json:"records"`
	Pct     float64 `json:"pct"`
}

type PeakDay struct {
	Date    string `json:"date"`
	Records uint64 `json:"records"`
}

type TopMetric struct {
	Name       string `json:"name"`
	Timeseries uint64 `json:"timeseries"`
}

// SummaryResponse powers the KPI strip, the by-type breakdown and the metrics pillar.
type SummaryResponse struct {
	Totals               SignalTotals `json:"totals"`
	ActiveTimeseries     uint64       `json:"activeTimeseries"`
	TopCardinalityMetric TopMetric    `json:"topCardinalityMetric"`
	DailyAverage         uint64       `json:"dailyAverage"`
	Peak                 PeakDay      `json:"peak"`
	DaysElapsed          int          `json:"daysElapsed"`
	DaysInMonth          int          `json:"daysInMonth"`
	ProjectedRecords     uint64       `json:"projectedRecords"`
	CommitmentRecords    uint64       `json:"commitmentRecords"`
	CommitmentUsedPct    float64      `json:"commitmentUsedPct"`
	ProjectedPct         float64      `json:"projectedPct"`
	OnPace               bool         `json:"onPace"`
	ByType               []TypeShare  `json:"byType"`
}

// TimeseriesSeries is one stacked band (a signal type or a service).
type TimeseriesSeries struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Data  []uint64 `json:"data"`
}

// TimeseriesResponse is the daily stacked series; all dates are actual (no projection).
type TimeseriesResponse struct {
	GroupBy string             `json:"groupBy"`
	Dates   []string           `json:"dates"`
	Series  []TimeseriesSeries `json:"series"`
}

// ServiceRow is one row of the top-ingesting-services table.
type ServiceRow struct {
	Name       string   `json:"name"`
	Env        string   `json:"env"`
	Logs       uint64   `json:"logs"`
	Spans      uint64   `json:"spans"`
	Timeseries uint64   `json:"timeseries"`
	Total      uint64   `json:"total"`
	Pct        float64  `json:"pct"`
	DeltaPct   float64  `json:"deltaPct"`
	Spark      []uint64 `json:"spark"`
}

// ServicesResponse is the full top-services table payload.
type ServicesResponse struct {
	Services      []ServiceRow `json:"services"`
	TotalServices int          `json:"totalServices"`
	TopSharePct   float64      `json:"topSharePct"`
}

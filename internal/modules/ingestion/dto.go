package ingestion

// Config holds operator-tunable settings for the ingestion usage view.
// The monthly commitments are the contracted ingestion budget the view compares
// against; they have no source in the telemetry store, so they live here as
// configurable constants (per-tenant/plan-based commitments are future work).
type Config struct {
	Enabled                 bool
	MonthlyRecordCommitment uint64
	MonthlyByteCommitment   uint64
}

func DefaultConfig() Config {
	return Config{
		Enabled:                 true,
		MonthlyRecordCommitment: 5_000_000_000,
		MonthlyByteCommitment:   50 * 1024 * 1024 * 1024 * 1024, // 50 TiB/month
	}
}

// SignalTotals carries the period record and byte counts per telemetry signal.
type SignalTotals struct {
	Logs             uint64 `json:"logs"`
	Spans            uint64 `json:"spans"`
	MetricDatapoints uint64 `json:"metricDatapoints"`
	Records          uint64 `json:"records"`
	LogsBytes        uint64 `json:"logsBytes"`
	SpansBytes       uint64 `json:"spansBytes"`
	MetricBytes      uint64 `json:"metricBytes"`
	Bytes            uint64 `json:"bytes"`
}

// TypeShare is one telemetry type's contribution to the period total.
type TypeShare struct {
	Type     string  `json:"type"`
	Label    string  `json:"label"`
	Records  uint64  `json:"records"`
	Pct      float64 `json:"pct"`
	Bytes    uint64  `json:"bytes"`
	BytesPct float64 `json:"bytesPct"`
}

type PeakDay struct {
	Date    string `json:"date"`
	Records uint64 `json:"records"`
	Bytes   uint64 `json:"bytes"`
}

type TopMetric struct {
	Name       string `json:"name"`
	Timeseries uint64 `json:"timeseries"`
}

// SummaryResponse powers the KPI strip, the by-type breakdown and the metrics
// pillar. Record- and byte-denominated facts sit side by side so the web unit
// toggle switches without a second round trip.
type SummaryResponse struct {
	Totals                 SignalTotals `json:"totals"`
	ActiveTimeseries       uint64       `json:"activeTimeseries"`
	TopCardinalityMetric   TopMetric    `json:"topCardinalityMetric"`
	DailyAverage           uint64       `json:"dailyAverage"`
	DailyAverageBytes      uint64       `json:"dailyAverageBytes"`
	Peak                   PeakDay      `json:"peak"`
	DaysElapsed            int          `json:"daysElapsed"`
	DaysInMonth            int          `json:"daysInMonth"`
	ProjectedRecords       uint64       `json:"projectedRecords"`
	ProjectedBytes         uint64       `json:"projectedBytes"`
	CommitmentRecords      uint64       `json:"commitmentRecords"`
	CommitmentBytes        uint64       `json:"commitmentBytes"`
	CommitmentUsedPct      float64      `json:"commitmentUsedPct"`
	CommitmentUsedBytesPct float64      `json:"commitmentUsedBytesPct"`
	ProjectedPct           float64      `json:"projectedPct"`
	ProjectedBytesPct      float64      `json:"projectedBytesPct"`
	OnPace                 bool         `json:"onPace"`
	OnPaceBytes            bool         `json:"onPaceBytes"`
	ByType                 []TypeShare  `json:"byType"`
}

// TimeseriesSeries is one stacked band (a signal type or a service). Data is
// records; ByteData is the same band denominated in bytes.
type TimeseriesSeries struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Data     []uint64 `json:"data"`
	ByteData []uint64 `json:"byteData"`
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
	Bytes      uint64   `json:"bytes"`
	Pct        float64  `json:"pct"`
	BytesPct   float64  `json:"bytesPct"`
	DeltaPct   float64  `json:"deltaPct"`
	Spark      []uint64 `json:"spark"`
	ByteSpark  []uint64 `json:"byteSpark"`
}

// ServicesResponse is the full top-services table payload.
type ServicesResponse struct {
	Services         []ServiceRow `json:"services"`
	TotalServices    int          `json:"totalServices"`
	TopSharePct      float64      `json:"topSharePct"`
	TopShareBytesPct float64      `json:"topShareBytesPct"`
}

package ingestion

import "github.com/optikklabs/query/internal/config"

type Config struct {
	MonthlyRecordCommitment uint64
	MonthlyByteCommitment   uint64

	PricePerGBLogsTraces         float64
	PricePerMillionMetricSamples float64
	Currency                     string
}

// NewConfig builds pricing from the billing config section; rates and
// the record commitment default via viper (billing.* keys).
func NewConfig(billing config.BillingConfig) Config {
	return Config{
		MonthlyRecordCommitment:      billing.MonthlyRecordCommitment,
		MonthlyByteCommitment:        50 * 1024 * 1024 * 1024 * 1024,
		PricePerGBLogsTraces:         billing.GBPriceUSD,
		PricePerMillionMetricSamples: billing.MetricMillionSamplesPriceUSD,
		Currency:                     "USD",
	}
}

func (c Config) Rates() Rates {
	return Rates{
		Currency:                c.Currency,
		PerGBLogsTraces:         c.PricePerGBLogsTraces,
		PerMillionMetricSamples: c.PricePerMillionMetricSamples,
	}
}

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

type SummaryResponse struct {
	Totals                 SignalTotals `json:"totals"`
	ActiveTimeseries       uint64       `json:"activeTimeseries"`
	TopCardinalityMetric   TopMetric    `json:"topCardinalityMetric"`
	DailyAverage           uint64       `json:"dailyAverage"`
	DailyAverageBytes      uint64       `json:"dailyAverageBytes"`
	Peak                   PeakDay      `json:"peak"`
	DaysElapsed            int          `json:"daysElapsed"`
	DaysInMonth            int          `json:"daysInMonth"`
	CommitmentRecords      uint64       `json:"commitmentRecords"`
	CommitmentBytes        uint64       `json:"commitmentBytes"`
	CommitmentUsedPct      float64      `json:"commitmentUsedPct"`
	CommitmentUsedBytesPct float64      `json:"commitmentUsedBytesPct"`
	ByType                 []TypeShare  `json:"byType"`
}

type TimeseriesSeries struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Data     []uint64 `json:"data"`
	ByteData []uint64 `json:"byteData"`
}

type TimeseriesResponse struct {
	GroupBy string             `json:"groupBy"`
	Dates   []string           `json:"dates"`
	Series  []TimeseriesSeries `json:"series"`
}

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

type ServicesResponse struct {
	Services         []ServiceRow `json:"services"`
	TotalServices    int          `json:"totalServices"`
	TopSharePct      float64      `json:"topSharePct"`
	TopShareBytesPct float64      `json:"topShareBytesPct"`
}

type OverviewResponse struct {
	Summary             SummaryResponse    `json:"summary"`
	Cost                CostResponse       `json:"cost"`
	TimeseriesByType    TimeseriesResponse `json:"timeseriesByType"`
	TimeseriesByService TimeseriesResponse `json:"timeseriesByService"`
	Services            ServicesResponse   `json:"services"`
	UsageSemantics      string             `json:"usageSemantics"`
}

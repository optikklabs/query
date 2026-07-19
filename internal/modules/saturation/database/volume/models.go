package volume

import "time"

type OpsTimeSeries struct {
	TimeBucket string   `json:"timeBucket"`
	GroupBy    string   `json:"groupBy"`
	OpsPerSec  *float64 `json:"opsPerSec"`
}

type opsRawDTO struct {
	TimeBucket time.Time `ch:"time_bucket"`
	GroupBy    string    `ch:"group_by"`
	OpsPerSec  float64   `ch:"ops_per_sec"`
}

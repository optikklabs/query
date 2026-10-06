package monitors

import (
	"time"

	"github.com/optikklabs/query/internal/shared/nullable"
)

type MonitorEventResponse struct {
	ID          int64      `json:"id"`
	MonitorID   int64      `json:"monitorId"`
	MonitorName string     `json:"monitorName"`
	Kind        string     `json:"kind"`
	Value       *float64   `json:"value,omitempty"`
	Threshold   *float64   `json:"threshold,omitempty"`
	PeakValue   *float64   `json:"peakValue,omitempty"`
	ResolvedBy  string     `json:"resolvedBy,omitempty"`
	Note        string     `json:"note,omitempty"`
	StartedAt   time.Time  `json:"startedAt"`
	EndedAt     *time.Time `json:"endedAt,omitempty"`
}

func toEventResponses(rows []EventRow) []MonitorEventResponse {
	out := make([]MonitorEventResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, MonitorEventResponse{
			ID:          r.ID,
			MonitorID:   r.MonitorID,
			MonitorName: r.MonitorName,
			Kind:        r.Kind,
			Value:       nullable.Ptr(r.Value.Float64, r.Value.Valid),
			Threshold:   nullable.Ptr(r.Threshold.Float64, r.Threshold.Valid),
			PeakValue:   nullable.Ptr(r.PeakValue.Float64, r.PeakValue.Valid),
			ResolvedBy:  r.ResolvedBy.String,
			Note:        r.Note.String,
			StartedAt:   r.StartedAt,
			EndedAt:     nullable.Ptr(r.EndedAt.Time, r.EndedAt.Valid),
		})
	}
	return out
}

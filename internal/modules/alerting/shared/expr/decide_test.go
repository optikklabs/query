package expr

import (
	"database/sql"
	"testing"
	"time"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

func TestDecideNoDataGrace(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	threshold := 10.0
	cond := models.Conditions{Comparator: "above", AlertThreshold: &threshold, NoDataAfterSec: 600, NoDataAs: "alert"}
	since := func(ago time.Duration) sql.NullTime { return sql.NullTime{Valid: true, Time: now.Add(-ago)} }

	tests := []struct {
		name            string
		prev            models.MonitorStateRow
		cond            models.Conditions
		hasData         bool
		wantStatus      string
		wantNoDataSince sql.NullTime
	}{
		{
			name:            "gap starts: hold status and record its start",
			prev:            models.MonitorStateRow{Status: "ok"},
			cond:            cond,
			wantStatus:      "ok",
			wantNoDataSince: since(0),
		},
		{
			name:            "gap within grace keeps original start",
			prev:            models.MonitorStateRow{Status: "ok", NoDataSince: since(5 * time.Minute)},
			cond:            cond,
			wantStatus:      "ok",
			wantNoDataSince: since(5 * time.Minute),
		},
		{
			name:            "gap past grace applies noDataAs",
			prev:            models.MonitorStateRow{Status: "ok", NoDataSince: since(10 * time.Minute)},
			cond:            cond,
			wantStatus:      "alert",
			wantNoDataSince: since(10 * time.Minute),
		},
		{
			name:            "zero grace applies noDataAs immediately",
			prev:            models.MonitorStateRow{Status: "ok"},
			cond:            models.Conditions{Comparator: "above", AlertThreshold: &threshold, NoDataAs: "alert"},
			wantStatus:      "alert",
			wantNoDataSince: since(0),
		},
		{
			name:       "data returning clears the gap",
			prev:       models.MonitorStateRow{Status: "ok", NoDataSince: since(5 * time.Minute)},
			cond:       cond,
			hasData:    true,
			wantStatus: "ok",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Decide(tt.prev, tt.cond, 1, tt.hasData, 0, now)
			if d.NewStatus != tt.wantStatus {
				t.Errorf("status = %q, want %q", d.NewStatus, tt.wantStatus)
			}
			if d.NoDataSince != tt.wantNoDataSince {
				t.Errorf("noDataSince = %+v, want %+v", d.NoDataSince, tt.wantNoDataSince)
			}
		})
	}
}
